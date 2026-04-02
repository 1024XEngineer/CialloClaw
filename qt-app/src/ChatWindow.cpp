#include "ChatWindow.h"

#include <QCloseEvent>
#include <QCoreApplication>
#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
#include <QNetworkRequest>
#include <QNetworkReply>
#include <QQmlContext>
#include <QUrl>

ChatWindow::ChatWindow(QWindow *parent)
    : QQuickView(parent), networkManager_(new QNetworkAccessManager(this)) {
    setFlags(Qt::Window | Qt::FramelessWindowHint);
    setResizeMode(QQuickView::SizeRootObjectToView);
    resize(420, 540);

    rootContext()->setContextProperty("chatWindow", this);
    setSource(QUrl::fromLocalFile(QCoreApplication::applicationDirPath() + "/resources/ChatWindow.qml"));

    title_ = "CialloClaw Chat";
    emit titleChanged();

    fetchChatInit();
}

ChatWindow::~ChatWindow() = default;

void ChatWindow::fetchChatInit() {
    QNetworkReply *reply = networkManager_->get(
        QNetworkRequest(QUrl("http://127.0.0.1:47831/api/chat/init")));
    connect(reply, &QNetworkReply::finished, this, &ChatWindow::onChatInitReply);
}

void ChatWindow::onChatInitReply() {
    auto *reply = qobject_cast<QNetworkReply *>(sender());
    if (!reply) {
        return;
    }

    if (reply->error() == QNetworkReply::NoError) {
        const QJsonDocument doc = QJsonDocument::fromJson(reply->readAll());
        const QJsonObject obj = doc.object();
        title_ = obj.value("title").toString("CialloClaw");
        messages_.clear();
        for (const auto &value : obj.value("messages").toArray()) {
            messages_.append(value.toString());
        }
        emit titleChanged();
        emit messagesChanged();
    }

    reply->deleteLater();
}

void ChatWindow::sendMessage(const QString &text) {
    if (text.trimmed().isEmpty()) {
        return;
    }

    messages_.append(QStringLiteral("你：") + text.trimmed());
    messages_.append(QStringLiteral("道奇兔：先记下啦，这里还是假的承接窗，但后面会接真实 Agent。"));
    emit messagesChanged();
}

void ChatWindow::startWindowMove() {
    startSystemMove();
}

void ChatWindow::hideWindow() {
    hide();
}

void ChatWindow::closeEvent(QCloseEvent *event) {
    event->ignore();
    hide();
}
