#include <QGuiApplication>
#include <QCoreApplication>
#include <QQmlApplicationEngine>
#include <QQmlContext>
#include <QQuickStyle>

#include "src/backendclient.h"

int main(int argc, char *argv[])
{
    QGuiApplication app(argc, argv);
    QCoreApplication::setApplicationName(QStringLiteral("CialloClaw Prototype 01 - 桌面天气机"));
    QCoreApplication::setOrganizationName(QStringLiteral("CialloClaw"));
    QQuickStyle::setStyle(QStringLiteral("Fusion"));

    BackendClient backend;

    QQmlApplicationEngine engine;
    engine.rootContext()->setContextProperty(QStringLiteral("backend"), &backend);
    engine.loadFromModule(QStringLiteral("CialloClawPrototype"), QStringLiteral("Main"));

    if (engine.rootObjects().isEmpty()) {
        return -1;
    }

    return app.exec();
}
