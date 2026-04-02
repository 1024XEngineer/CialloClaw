#include <QApplication>
#include <QDir>
#include <QFileInfo>
#include <QProcess>
#include "PetWindow.h"
#include "ChatWindow.h"
#include "SettingsWindow.h"

int main(int argc, char *argv[]) {
    QApplication app(argc, argv);
    app.setApplicationName("CialloClaw");
    app.setOrganizationName("CialloClaw");

    const QString appDir = QCoreApplication::applicationDirPath();
    const QString projectRoot = QDir(appDir).absoluteFilePath("..");
    const QString goBackendDir = QDir(projectRoot).absoluteFilePath("../go-backend");
    const QString sidecarExe = QDir(goBackendDir).absoluteFilePath("bin/cialloclaw-sidecar.exe");

    // Start Go sidecar
    QProcess *sidecar = new QProcess(&app);
    if (QFileInfo::exists(sidecarExe)) {
        sidecar->setProgram(sidecarExe);
        sidecar->setWorkingDirectory(goBackendDir);
    } else {
        sidecar->setProgram("go");
        sidecar->setArguments({"run", "./cmd/sidecar/main.go"});
        sidecar->setWorkingDirectory(goBackendDir);
    }
    sidecar->start();

    // Create pet window
    PetWindow *pet = new PetWindow();
    pet->show();

    // Create chat window
    ChatWindow *chat = new ChatWindow();
    chat->hide();

    // Create settings window
    SettingsWindow *settings = new SettingsWindow();
    settings->hide();

    // Connect pet to chat
    QObject::connect(pet, &PetWindow::openChatRequested, [chat]() {
        if (chat->isVisible()) {
            chat->hide();
        } else {
            chat->show();
            chat->raise();
            chat->requestActivate();
        }
    });

    QObject::connect(pet, &PetWindow::openSettingsRequested, [settings]() {
        settings->show();
        settings->raise();
        settings->activateWindow();
    });

    QObject::connect(pet, &PetWindow::hideRequested, [pet, chat, settings]() {
        pet->hide();
        chat->hide();
        settings->hide();
    });

    QObject::connect(pet, &PetWindow::quitRequested, [&app]() {
        app.quit();
    });

    return app.exec();
}
