#include "PetWindow.h"

#include <QApplication>
#include <QBitmap>
#include <QContextMenuEvent>
#include <QJsonDocument>
#include <QJsonObject>
#include <QNetworkRequest>
#include <QNetworkReply>
#include <QPainter>

namespace {
constexpr int kSpriteSize = 20;
constexpr int kScale = 5;

QColor spriteColor(QChar ch) {
    switch (ch.unicode()) {
    case 'k': return QColor(8, 8, 12);        // black fur
    case 'g': return QColor(94, 98, 110);     // gray highlight
    case 'b': return QColor(118, 126, 255);   // blue eyes
    case 'm': return QColor(96, 100, 108);    // mouth line
    default: return Qt::transparent;
    }
}

QImage buildRabbitSprite() {
    static const char *rows[kSpriteSize] = {
        "........kk..kk......",
        "........kg..gk......",
        "........kg..gk......",
        "........kg..gk......",
        ".......kkkkkkkk.....",
        ".......kkkkkkkkk....",
        "......kkkkkkkkgk....",
        "......kkkkkkkggkk...",
        "......kkbk.kbggkk...",
        "......kkkkkkkkggkk..",
        "......kkkkmkkgggkk..",
        "......kkkkkkkkgggkk.",
        ".......kkkkkkkggkkk.",
        ".......kkkkkkkkggkk.",
        "........kkkkkkkkkk..",
        "........kkk..kkkk...",
        "........kk....kkk...",
        "..............gkk...",
        "..............kk....",
        "...................."
    };

    QImage image(kSpriteSize, kSpriteSize, QImage::Format_ARGB32);
    image.fill(Qt::transparent);

    for (int y = 0; y < kSpriteSize; ++y) {
        for (int x = 0; x < kSpriteSize; ++x) {
            const QColor color = spriteColor(QChar(rows[y][x]));
            image.setPixelColor(x, y, color);
        }
    }
    return image;
}
} // namespace

PetWindow::PetWindow(QWidget *parent)
    : QWidget(parent, Qt::FramelessWindowHint | Qt::WindowStaysOnTopHint | Qt::Tool)
    , contextMenu_(new QMenu(this))
    , trayIcon_(nullptr)
    , trayMenu_(nullptr)
    , networkManager_(new QNetworkAccessManager(this))
    , isDragging_(false) {
    setAttribute(Qt::WA_TranslucentBackground);
    setAttribute(Qt::WA_DeleteOnClose, false);
    setFixedSize(kSpriteSize * kScale, kSpriteSize * kScale);

    contextMenu_->addAction(QStringLiteral("设置"), this, &PetWindow::openSettingsRequested);
    contextMenu_->addAction(QStringLiteral("隐藏"), this, &PetWindow::hideRequested);
    contextMenu_->addAction(QStringLiteral("退出"), this, &PetWindow::quitRequested);

    rebuildSprite();
    setupTray();
    setupMask();
    fetchPetStatus();
}

PetWindow::~PetWindow() = default;

void PetWindow::rebuildSprite() {
    const QImage sprite = buildRabbitSprite();
    petPixmap_ = QPixmap::fromImage(sprite.scaled(size(), Qt::IgnoreAspectRatio, Qt::FastTransformation));

    if (trayIcon_) {
        trayIcon_->setIcon(QIcon(petPixmap_));
    }
}

void PetWindow::setupMask() {
    if (petPixmap_.isNull()) {
        rebuildSprite();
    }
    setMask(petPixmap_.mask());
}

void PetWindow::setupTray() {
    trayMenu_ = new QMenu(this);
    trayMenu_->addAction(QStringLiteral("显示"), this, [this]() {
        show();
        raise();
        activateWindow();
    });
    trayMenu_->addAction(QStringLiteral("退出"), this, &PetWindow::quitRequested);

    trayIcon_ = new QSystemTrayIcon(QIcon(petPixmap_), this);
    trayIcon_->setContextMenu(trayMenu_);
    trayIcon_->setToolTip(QStringLiteral("CialloClaw"));
    trayIcon_->show();

    connect(trayIcon_, &QSystemTrayIcon::activated, this, &PetWindow::onTrayActivated);
}

void PetWindow::fetchPetStatus() {
    auto *reply = networkManager_->get(QNetworkRequest(QUrl(QStringLiteral("http://127.0.0.1:47831/api/pet"))));
    connect(reply, &QNetworkReply::finished, this, &PetWindow::onPetStatusReply);
}

void PetWindow::onPetStatusReply() {
    auto *reply = qobject_cast<QNetworkReply *>(sender());
    if (!reply) {
        return;
    }

    if (reply->error() == QNetworkReply::NoError) {
        const auto obj = QJsonDocument::fromJson(reply->readAll()).object();
        setToolTip(obj.value(QStringLiteral("mood")).toString(QStringLiteral("伴")));
    }

    reply->deleteLater();
}

void PetWindow::mousePressEvent(QMouseEvent *event) {
    if (event->button() == Qt::LeftButton) {
        dragStartPosition_ = event->globalPosition().toPoint() - frameGeometry().topLeft();
        isDragging_ = false;
        event->accept();
    }
}

void PetWindow::mouseMoveEvent(QMouseEvent *event) {
    if (event->buttons() & Qt::LeftButton) {
        const QPoint newPos = event->globalPosition().toPoint() - dragStartPosition_;
        if ((newPos - pos()).manhattanLength() > QApplication::startDragDistance()) {
            isDragging_ = true;
        }
        move(newPos);
    }
}

void PetWindow::mouseReleaseEvent(QMouseEvent *event) {
    if (event->button() == Qt::LeftButton && !isDragging_) {
        emit openChatRequested();
    }
    isDragging_ = false;
}

void PetWindow::contextMenuEvent(QContextMenuEvent *event) {
    showContextMenu(event->globalPos());
}

void PetWindow::showContextMenu(const QPoint &pos) {
    contextMenu_->exec(pos);
}

void PetWindow::onTrayActivated(QSystemTrayIcon::ActivationReason reason) {
    if (reason == QSystemTrayIcon::Trigger) {
        show();
        raise();
        activateWindow();
    }
}

void PetWindow::paintEvent(QPaintEvent *) {
    QPainter painter(this);
    painter.setRenderHint(QPainter::Antialiasing, false);
    painter.drawPixmap(0, 0, petPixmap_);
}
