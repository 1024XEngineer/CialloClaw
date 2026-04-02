#include "SettingsWindow.h"

#include <QCheckBox>
#include <QCloseEvent>
#include <QLabel>
#include <QPushButton>
#include <QVBoxLayout>

SettingsWindow::SettingsWindow(QWidget *parent) : QWidget(parent) {
    setWindowTitle(QStringLiteral("CialloClaw 设置"));
    resize(320, 320);

    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(18, 18, 18, 18);
    layout->setSpacing(12);

    auto *title = new QLabel(QStringLiteral("静态设置"), this);
    title->setStyleSheet("font-size: 20px; font-weight: 700; color: #1f2430;");
    layout->addWidget(title);

    auto *themeButton = new QPushButton(QStringLiteral("主题：奶油像素"), this);
    auto *petButton = new QPushButton(QStringLiteral("桌宠尺寸：标准"), this);
    auto *chatButton = new QPushButton(QStringLiteral("对话窗口：浅色 mock"), this);
    auto *aboutButton = new QPushButton(QStringLiteral("关于：CialloClaw Qt 原型"), this);
    auto *trayCheck = new QCheckBox(QStringLiteral("启用托盘恢复"), this);
    trayCheck->setChecked(true);

    const QString buttonStyle = QStringLiteral(
        "QPushButton { background:#fff7ea; border:2px solid #1f2430; border-radius:10px; padding:10px; text-align:left; }"
        "QPushButton:hover { background:#f4ead9; }"
        "QCheckBox { color:#1f2430; font-size:14px; }"
    );

    themeButton->setStyleSheet(buttonStyle);
    petButton->setStyleSheet(buttonStyle);
    chatButton->setStyleSheet(buttonStyle);
    aboutButton->setStyleSheet(buttonStyle);
    trayCheck->setStyleSheet(buttonStyle);

    layout->addWidget(themeButton);
    layout->addWidget(petButton);
    layout->addWidget(chatButton);
    layout->addWidget(aboutButton);
    layout->addWidget(trayCheck);
    layout->addStretch();
}

SettingsWindow::~SettingsWindow() = default;

void SettingsWindow::closeEvent(QCloseEvent *event) {
    event->ignore();
    hide();
}
