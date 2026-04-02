#pragma once

#include <QJsonObject>
#include <QNetworkAccessManager>
#include <QObject>
#include <QUrl>
#include <QUrlQuery>
#include <QVariant>
#include <QVariantList>
#include <QVariantMap>
#include <functional>

class BackendClient : public QObject
{
    Q_OBJECT
    Q_PROPERTY(QString baseUrl READ baseUrl WRITE setBaseUrl NOTIFY baseUrlChanged)
    Q_PROPERTY(bool connected READ connected NOTIFY connectedChanged)
    Q_PROPERTY(QString currentSceneId READ currentSceneId NOTIFY currentSceneIdChanged)
    Q_PROPERTY(QVariantMap bootstrap READ bootstrapData NOTIFY bootstrapChanged)
    Q_PROPERTY(QVariantList scenes READ scenes NOTIFY scenesChanged)
    Q_PROPERTY(QVariantMap currentScene READ currentScene NOTIFY currentSceneChanged)
    Q_PROPERTY(QVariantMap currentWeather READ currentWeather NOTIFY currentWeatherChanged)
    Q_PROPERTY(QVariantMap currentSituation READ currentSituation NOTIFY currentSituationChanged)
    Q_PROPERTY(QVariantMap memory READ memory NOTIFY memoryChanged)
    Q_PROPERTY(QVariantList logs READ logs NOTIFY logsChanged)
    Q_PROPERTY(QVariantMap pendingRiskPreview READ pendingRiskPreview NOTIFY pendingRiskPreviewChanged)
    Q_PROPERTY(QString lastNotice READ lastNotice NOTIFY lastNoticeChanged)

public:
    explicit BackendClient(QObject *parent = nullptr);

    QString baseUrl() const;
    void setBaseUrl(const QString &url);

    bool connected() const;
    QString currentSceneId() const;
    QVariantMap bootstrapData() const;
    QVariantList scenes() const;
    QVariantMap currentScene() const;
    QVariantMap currentWeather() const;
    QVariantMap currentSituation() const;
    QVariantMap memory() const;
    QVariantList logs() const;
    QVariantMap pendingRiskPreview() const;
    QString lastNotice() const;

    Q_INVOKABLE void bootstrap();
    Q_INVOKABLE void selectScene(const QString &sceneId);
    Q_INVOKABLE void proceed(const QString &actionKey);
    Q_INVOKABLE void requestRiskPreview(const QString &actionKey);
    Q_INVOKABLE void authorize(const QString &actionKey, const QString &depthKey);
    Q_INVOKABLE void loadMemory();
    Q_INVOKABLE void loadLogs();

signals:
    void baseUrlChanged();
    void connectedChanged();
    void currentSceneIdChanged();
    void bootstrapChanged();
    void scenesChanged();
    void currentSceneChanged();
    void currentWeatherChanged();
    void currentSituationChanged();
    void memoryChanged();
    void logsChanged();
    void pendingRiskPreviewChanged();
    void lastNoticeChanged();
    void proceedCompleted(const QVariantMap &result);
    void riskPreviewCompleted(const QVariantMap &result);
    void authorizationCompleted(const QVariantMap &result);

private:
    using ApiCallback = std::function<void(bool ok, const QVariant &data, const QString &message, const QJsonObject &root)>;

    void request(const QString &path, QNetworkAccessManager::Operation op, const QVariantMap &body, const QUrlQuery &query, const ApiCallback &cb);
    QUrl urlFor(const QString &path, const QUrlQuery &query = QUrlQuery()) const;
    QVariantMap sceneSummaryMap(const QString &sceneId) const;
    QVariantMap makePlaceholderScene(const QVariantMap &summary) const;
    QVariantMap makePlaceholderWeather(const QVariantMap &summary) const;
    QVariantMap makePlaceholderSituation(const QVariantMap &summary) const;
    QVariantMap asMap(const QVariant &value) const;
    QVariantList asList(const QVariant &value) const;
    void setConnected(bool value);
    void setLastNotice(const QString &value);
    void setBootstrap(const QVariantMap &value);
    void setScenes(const QVariantList &value);
    void setCurrentScene(const QVariantMap &value);
    void setCurrentWeather(const QVariantMap &value);
    void setCurrentSituation(const QVariantMap &value);
    void setMemory(const QVariantMap &value);
    void setLogs(const QVariantList &value);
    void setPendingRiskPreview(const QVariantMap &value);
    QString normalizeBaseUrl(QString value) const;

    QVariantMap sceneActionMap(const QVariantMap &scene) const;
    QString textureForState(const QString &state) const;

    QNetworkAccessManager m_network;
    QString m_baseUrl;
    bool m_connected = false;
    QString m_currentSceneId;
    QVariantMap m_bootstrap;
    QVariantList m_scenes;
    QVariantMap m_currentScene;
    QVariantMap m_currentWeather;
    QVariantMap m_currentSituation;
    QVariantMap m_memory;
    QVariantList m_logs;
    QVariantMap m_pendingRiskPreview;
    QString m_lastNotice;
};
