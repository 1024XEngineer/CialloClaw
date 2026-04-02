#include "backendclient.h"

#include <QJsonDocument>
#include <QNetworkReply>
#include <QNetworkRequest>
#include <QUrlQuery>
#include <QVariant>

namespace {
	static QVariantMap mapFromVariant(const QVariant &value)
	{
		return value.toMap();
	}

	static QVariantList listFromVariant(const QVariant &value)
	{
		return value.toList();
	}

}

BackendClient::BackendClient(QObject *parent)
    : QObject(parent)
    , m_baseUrl(normalizeBaseUrl(qEnvironmentVariable("CIALLO_CLAW_BACKEND_URL", QStringLiteral("http://127.0.0.1:17888"))))
    , m_lastNotice(QStringLiteral("正在连接本地天气机…"))
{
}

QString BackendClient::baseUrl() const
{
	return m_baseUrl;
}

void BackendClient::setBaseUrl(const QString &url)
{
	const QString normalized = normalizeBaseUrl(url);
	if (normalized == m_baseUrl) {
		return;
	}
	m_baseUrl = normalized;
	emit baseUrlChanged();
}

bool BackendClient::connected() const
{
	return m_connected;
}

QString BackendClient::currentSceneId() const
{
	return m_currentSceneId;
}

QVariantMap BackendClient::bootstrapData() const
{
	return m_bootstrap;
}

QVariantList BackendClient::scenes() const
{
	return m_scenes;
}

QVariantMap BackendClient::currentScene() const
{
	return m_currentScene;
}

QVariantMap BackendClient::currentWeather() const
{
	return m_currentWeather;
}

QVariantMap BackendClient::currentSituation() const
{
	return m_currentSituation;
}

QVariantMap BackendClient::memory() const
{
	return m_memory;
}

QVariantList BackendClient::logs() const
{
	return m_logs;
}

QVariantMap BackendClient::pendingRiskPreview() const
{
	return m_pendingRiskPreview;
}

QString BackendClient::lastNotice() const
{
	return m_lastNotice;
}

void BackendClient::bootstrap()
{
	request(QStringLiteral("/api/bootstrap"), QNetworkAccessManager::GetOperation, {}, {}, [this](bool ok, const QVariant &data, const QString &message, const QJsonObject &) {
		if (!ok) {
			setConnected(false);
			if (!message.isEmpty()) {
				setLastNotice(message);
			}
			return;
		}

		const QVariantMap bootstrapMap = mapFromVariant(data);
		setBootstrap(bootstrapMap);
		setScenes(listFromVariant(bootstrapMap.value(QStringLiteral("scenes"))));
		if (!message.isEmpty()) {
			setLastNotice(message);
		}

		const QString defaultSceneId = bootstrapMap.value(QStringLiteral("defaultSceneId")).toString();
		if (!defaultSceneId.isEmpty()) {
			selectScene(defaultSceneId);
		}
		loadMemory();
		loadLogs();
	});
}

void BackendClient::selectScene(const QString &sceneId)
{
	if (sceneId.isEmpty()) {
		return;
	}

	m_currentSceneId = sceneId;
	emit currentSceneIdChanged();

	const QVariantMap summary = sceneSummaryMap(sceneId);
	if (!summary.isEmpty()) {
		setCurrentScene(makePlaceholderScene(summary));
		setCurrentWeather(makePlaceholderWeather(summary));
		setCurrentSituation(makePlaceholderSituation(summary));
		if (!summary.value(QStringLiteral("subtitle")).toString().isEmpty()) {
			setLastNotice(summary.value(QStringLiteral("subtitle")).toString());
		}
	}

	request(QStringLiteral("/api/scenes/%1").arg(sceneId), QNetworkAccessManager::GetOperation, {}, {}, [this, sceneId](bool ok, const QVariant &data, const QString &message, const QJsonObject &) {
		if (sceneId != m_currentSceneId) {
			return;
		}
		if (!ok) {
			if (!message.isEmpty()) {
				setLastNotice(message);
			}
			return;
		}
		setCurrentScene(mapFromVariant(data));
		if (!message.isEmpty()) {
			setLastNotice(message);
		}
	});

	QUrlQuery weatherQuery;
	weatherQuery.addQueryItem(QStringLiteral("scene"), sceneId);
	request(QStringLiteral("/api/weather/current"), QNetworkAccessManager::GetOperation, {}, weatherQuery, [this, sceneId](bool ok, const QVariant &data, const QString &message, const QJsonObject &) {
		if (sceneId != m_currentSceneId) {
			return;
		}
		if (!ok) {
			if (!message.isEmpty()) {
				setLastNotice(message);
			}
			return;
		}
		setCurrentWeather(mapFromVariant(data));
		if (!message.isEmpty()) {
			setLastNotice(message);
		}
	});

	request(QStringLiteral("/api/situation/%1").arg(sceneId), QNetworkAccessManager::GetOperation, {}, {}, [this, sceneId](bool ok, const QVariant &data, const QString &message, const QJsonObject &) {
		if (sceneId != m_currentSceneId) {
			return;
		}
		if (!ok) {
			if (!message.isEmpty()) {
				setLastNotice(message);
			}
			return;
		}
		setCurrentSituation(mapFromVariant(data));
		if (!message.isEmpty()) {
			setLastNotice(message);
		}
	});
}

void BackendClient::proceed(const QString &actionKey)
{
	QVariantMap body;
	body.insert(QStringLiteral("scene"), m_currentSceneId);
	body.insert(QStringLiteral("action"), actionKey);
	request(QStringLiteral("/api/action/proceed"), QNetworkAccessManager::PostOperation, body, {}, [this](bool ok, const QVariant &data, const QString &message, const QJsonObject &) {
		if (!ok) {
			if (!message.isEmpty()) {
				setLastNotice(message);
			}
			return;
		}
		const QVariantMap result = mapFromVariant(data);
		if (!message.isEmpty()) {
			setLastNotice(message);
		}
		emit proceedCompleted(result);
	});
}

void BackendClient::requestRiskPreview(const QString &actionKey)
{
	QVariantMap body;
	body.insert(QStringLiteral("scene"), m_currentSceneId);
	body.insert(QStringLiteral("action"), actionKey);
	request(QStringLiteral("/api/action/risk-preview"), QNetworkAccessManager::PostOperation, body, {}, [this](bool ok, const QVariant &data, const QString &message, const QJsonObject &) {
		if (!ok) {
			if (!message.isEmpty()) {
				setLastNotice(message);
			}
			return;
		}
		const QVariantMap result = mapFromVariant(data);
		setPendingRiskPreview(result);
		if (!message.isEmpty()) {
			setLastNotice(message);
		}
		emit riskPreviewCompleted(result);
	});
}

void BackendClient::authorize(const QString &actionKey, const QString &depthKey)
{
	QVariantMap body;
	body.insert(QStringLiteral("scene"), m_currentSceneId);
	body.insert(QStringLiteral("action"), actionKey);
	body.insert(QStringLiteral("depth"), depthKey);
	request(QStringLiteral("/api/action/authorize"), QNetworkAccessManager::PostOperation, body, {}, [this](bool ok, const QVariant &data, const QString &message, const QJsonObject &) {
		if (!ok) {
			if (!message.isEmpty()) {
				setLastNotice(message);
			}
			return;
		}
		const QVariantMap result = mapFromVariant(data);
		if (!message.isEmpty()) {
			setLastNotice(message);
		}
		emit authorizationCompleted(result);
	});
}

void BackendClient::loadMemory()
{
	request(QStringLiteral("/api/memory"), QNetworkAccessManager::GetOperation, {}, {}, [this](bool ok, const QVariant &data, const QString &message, const QJsonObject &) {
		if (!ok) {
			return;
		}
		setMemory(mapFromVariant(data));
		if (!message.isEmpty()) {
			setLastNotice(message);
		}
	});
}

void BackendClient::loadLogs()
{
	request(QStringLiteral("/api/logs"), QNetworkAccessManager::GetOperation, {}, {}, [this](bool ok, const QVariant &data, const QString &message, const QJsonObject &) {
		if (!ok) {
			return;
		}
		setLogs(listFromVariant(data));
		if (!message.isEmpty()) {
			setLastNotice(message);
		}
	});
}

void BackendClient::request(const QString &path, QNetworkAccessManager::Operation op, const QVariantMap &body, const QUrlQuery &query, const ApiCallback &cb)
{
	QNetworkRequest req(urlFor(path, query));
	req.setHeader(QNetworkRequest::ContentTypeHeader, QByteArrayLiteral("application/json; charset=utf-8"));

	QNetworkReply *reply = nullptr;
	if (op == QNetworkAccessManager::GetOperation) {
		reply = m_network.get(req);
	} else if (op == QNetworkAccessManager::PostOperation) {
		const QByteArray payload = QJsonDocument::fromVariant(body).toJson(QJsonDocument::Compact);
		reply = m_network.post(req, payload);
	} else {
		setLastNotice(QStringLiteral("不支持的 HTTP 操作"));
		setConnected(false);
		return;
	}

	connect(reply, &QNetworkReply::finished, this, [this, reply, cb]() {
		const int statusCode = reply->attribute(QNetworkRequest::HttpStatusCodeAttribute).toInt();
		const bool transportOK = statusCode > 0;
		if (transportOK) {
			setConnected(true);
		}

		const QByteArray raw = reply->readAll();
		QJsonParseError error{};
		const QJsonDocument doc = QJsonDocument::fromJson(raw, &error);
		QJsonObject root;
		if (doc.isObject()) {
			root = doc.object();
		}

		if (!transportOK) {
			const QString message = root.value(QStringLiteral("message")).toString().isEmpty() ? reply->errorString() : root.value(QStringLiteral("message")).toString();
			setConnected(false);
			setLastNotice(message);
			cb(false, QVariant(), message, root);
			reply->deleteLater();
			return;
		}

		if (error.error != QJsonParseError::NoError && !raw.trimmed().isEmpty() && root.isEmpty()) {
			const QString message = QStringLiteral("JSON 解析失败：%1").arg(error.errorString());
			setLastNotice(message);
			cb(false, QVariant(), message, root);
			reply->deleteLater();
			return;
		}

		const bool ok = root.contains(QStringLiteral("ok")) ? root.value(QStringLiteral("ok")).toBool(false) : true;
		const QString message = root.value(QStringLiteral("message")).toString();
		const QVariant data = root.value(QStringLiteral("data")).toVariant();
		if (!message.isEmpty()) {
			setLastNotice(message);
		}
		cb(ok, data, message, root);
		reply->deleteLater();
	});
}

QUrl BackendClient::urlFor(const QString &path, const QUrlQuery &query) const
{
	QUrl url(m_baseUrl + path);
	if (!query.isEmpty()) {
		url.setQuery(query);
	}
	return url;
}

QVariantMap BackendClient::sceneSummaryMap(const QString &sceneId) const
{
	for (const QVariant &item : m_scenes) {
		const QVariantMap map = item.toMap();
		if (map.value(QStringLiteral("id")).toString() == sceneId) {
			return map;
		}
	}
	return {};
}

QVariantMap BackendClient::sceneActionMap(const QVariantMap &scene) const
{
	return scene.value(QStringLiteral("primaryAction")).toMap();
}

QVariantMap BackendClient::makePlaceholderScene(const QVariantMap &summary) const
{
	QVariantMap scene;
	scene.insert(QStringLiteral("id"), summary.value(QStringLiteral("id")));
	scene.insert(QStringLiteral("title"), summary.value(QStringLiteral("title")));
	scene.insert(QStringLiteral("subtitle"), summary.value(QStringLiteral("subtitle")));
	scene.insert(QStringLiteral("sceneType"), summary.value(QStringLiteral("sceneType")));
	scene.insert(QStringLiteral("weather"), makePlaceholderWeather(summary));
	scene.insert(QStringLiteral("storyLead"), summary.value(QStringLiteral("subtitle")));
	scene.insert(QStringLiteral("summary"), summary.value(QStringLiteral("subtitle")));
	scene.insert(QStringLiteral("tone"), summary.value(QStringLiteral("tone")));
	scene.insert(QStringLiteral("accent"), summary.value(QStringLiteral("accent")));
	scene.insert(QStringLiteral("primaryAction"), sceneActionMap(summary));
	scene.insert(QStringLiteral("content"), QVariantMap{});
	return scene;
}

QVariantMap BackendClient::makePlaceholderWeather(const QVariantMap &summary) const
{
	const QString state = summary.value(QStringLiteral("weatherState")).toString();
	const QString label = summary.value(QStringLiteral("weatherLabel")).toString();
	QVariantMap weather;
	weather.insert(QStringLiteral("state"), state);
	weather.insert(QStringLiteral("label"), label);
	weather.insert(QStringLiteral("hint"), QStringLiteral("正在接入这个场景的天气细节…"));
	weather.insert(QStringLiteral("bandText"), summary.value(QStringLiteral("title")).toString() + QStringLiteral(" · 细节接入中"));
	weather.insert(QStringLiteral("tone"), summary.value(QStringLiteral("tone")));
	weather.insert(QStringLiteral("accent"), summary.value(QStringLiteral("accent")));
	weather.insert(QStringLiteral("texture"), textureForState(state));
	return weather;
}

QVariantMap BackendClient::makePlaceholderSituation(const QVariantMap &summary) const
{
	QVariantMap situation;
	situation.insert(QStringLiteral("sceneId"), summary.value(QStringLiteral("id")));
	situation.insert(QStringLiteral("weather"), makePlaceholderWeather(summary));
	situation.insert(QStringLiteral("summary"), QStringLiteral("正在整理此场景的今日态势图…"));
	situation.insert(QStringLiteral("topPriorities"), QVariantList{});
	situation.insert(QStringLiteral("prepared"), QVariantList{});
	situation.insert(QStringLiteral("habitBasis"), summary.value(QStringLiteral("subtitle")));
	situation.insert(QStringLiteral("actionLabel"), sceneActionMap(summary).value(QStringLiteral("label")));
	situation.insert(QStringLiteral("pressure"), summary.value(QStringLiteral("weatherLabel")));
	return situation;
}

QVariantMap BackendClient::asMap(const QVariant &value) const
{
	return value.toMap();
}

QVariantList BackendClient::asList(const QVariant &value) const
{
	return value.toList();
}

void BackendClient::setConnected(bool value)
{
	if (m_connected == value) {
		return;
	}
	m_connected = value;
	emit connectedChanged();
}

void BackendClient::setLastNotice(const QString &value)
{
	if (m_lastNotice == value) {
		return;
	}
	m_lastNotice = value;
	emit lastNoticeChanged();
}

void BackendClient::setBootstrap(const QVariantMap &value)
{
	m_bootstrap = value;
	emit bootstrapChanged();
}

void BackendClient::setScenes(const QVariantList &value)
{
	m_scenes = value;
	emit scenesChanged();
}

void BackendClient::setCurrentScene(const QVariantMap &value)
{
	m_currentScene = value;
	emit currentSceneChanged();
}

void BackendClient::setCurrentWeather(const QVariantMap &value)
{
	m_currentWeather = value;
	emit currentWeatherChanged();
}

void BackendClient::setCurrentSituation(const QVariantMap &value)
{
	m_currentSituation = value;
	emit currentSituationChanged();
}

void BackendClient::setMemory(const QVariantMap &value)
{
	m_memory = value;
	emit memoryChanged();
}

void BackendClient::setLogs(const QVariantList &value)
{
	m_logs = value;
	emit logsChanged();
}

void BackendClient::setPendingRiskPreview(const QVariantMap &value)
{
	m_pendingRiskPreview = value;
	emit pendingRiskPreviewChanged();
}

QString BackendClient::normalizeBaseUrl(QString value) const
{
	value = value.trimmed();
	if (value.isEmpty()) {
		value = QStringLiteral("http://127.0.0.1:17888");
	}
	if (!value.startsWith(QStringLiteral("http://")) && !value.startsWith(QStringLiteral("https://"))) {
		value.prepend(QStringLiteral("http://"));
	}
	while (value.endsWith('/')) {
		value.chop(1);
	}
	return value;
}

QString BackendClient::textureForState(const QString &state) const
{
	if (state == QStringLiteral("sunny")) {
		return QStringLiteral("clear");
	}
	if (state == QStringLiteral("fog")) {
		return QStringLiteral("mist");
	}
	if (state == QStringLiteral("storm")) {
		return QStringLiteral("storm");
	}
	if (state == QStringLiteral("shower")) {
		return QStringLiteral("shower");
	}
	return QStringLiteral("cloud");
}
