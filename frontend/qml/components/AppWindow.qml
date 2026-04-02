import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

ApplicationWindow {
    id: root
    width: 1480
    height: 980
    minimumWidth: 1280
    minimumHeight: 860
    visible: true
    title: qsTr("CialloClaw Prototype 01 - 桌面天气机")
    color: "#06111E"

    property bool situationOpen: false
    property bool stormOpen: false
    property bool draftVisible: false
    property var runtimeDraft: ({})
    property var runtimeRiskPreview: ({})

    property string backendUrl: "http://127.0.0.1:17888"

    property var startupWeather: ({
        state: "cloudy",
        label: "多云",
        hint: "正在连接本地天气机，默认进入多云状态。",
        bandText: "默认进入多云状态",
        tone: "轻压但稳定",
        accent: "#9CB7D8",
        texture: "cloud"
    })

    property var startupScene: ({
        id: "",
        title: "桌面天气机",
        subtitle: "正在连接本地场景…",
        sceneType: "loading",
        weather: startupWeather,
        storyLead: "",
        summary: "",
        tone: "",
        accent: "#9CB7D8",
        primaryAction: ({ label: "推进一步", key: "", type: "proceed", description: "" }),
        content: ({})
    })

    property var startupSituation: ({
        sceneId: "",
        weather: startupWeather,
        summary: "正在整理今日态势图…",
        topPriorities: [],
        prepared: [],
        habitBasis: "",
        actionLabel: "推进一步",
        pressure: "多云"
    })

    property var startupMemory: ({
        headline: "镜子记忆",
        note: "正在读取你的习惯镜像…",
        habits: [],
        updatedAt: "",
        confidence: ""
    })

    QtObject {
        id: backend

        property bool connected: false
        property string currentSceneId: ""
        property var bootstrapData: ({})
        property var scenes: []
        property var currentScene: ({})
        property var currentWeather: ({})
        property var currentSituation: ({})
        property var memory: ({})
        property var logs: []
        property var pendingRiskPreview: ({})
        property string lastNotice: "正在连接本地天气机…"

        signal proceedCompleted(var result)
        signal riskPreviewCompleted(var result)
        signal authorizationCompleted(var result)

        function setLastNotice(text) {
            if (lastNotice === text) {
                return
            }
            lastNotice = text
        }

        function setConnected(value) {
            if (connected === value) {
                return
            }
            connected = value
        }

        function request(path, method, body, callback) {
            const xhr = new XMLHttpRequest()
            xhr.open(method, root.backendUrl + path)
            xhr.setRequestHeader("Content-Type", "application/json; charset=utf-8")
            xhr.onreadystatechange = function() {
                if (xhr.readyState !== XMLHttpRequest.DONE) {
                    return
                }

                let rootJson = null
                let ok = false
                let data = null
                let message = ""

                try {
                    rootJson = xhr.responseText ? JSON.parse(xhr.responseText) : null
                } catch (e) {
                    rootJson = null
                }

                if (xhr.status >= 200 && xhr.status < 300 && rootJson) {
                    ok = !!rootJson.ok
                    message = rootJson.message || ""
                    data = rootJson.data
                    setConnected(true)
                } else {
                    ok = false
                    message = rootJson && rootJson.message ? rootJson.message : (xhr.status ? ("HTTP " + xhr.status) : "网络请求失败")
                    setConnected(false)
                }

                if (message) {
                    setLastNotice(message)
                }
                if (callback) {
                    callback(ok, data, message, rootJson)
                }
            }

            xhr.send(body ? JSON.stringify(body) : "")
        }

        function bootstrap() {
            request("/api/bootstrap", "GET", null, function(ok, data, message) {
                if (!ok || !data) {
                    return
                }
                bootstrapData = data
                scenes = data.scenes || []
                const defaultSceneId = data.defaultSceneId || ""
                if (defaultSceneId) {
                    selectScene(defaultSceneId)
                }
                loadMemory()
                loadLogs()
                if (message) {
                    setLastNotice(message)
                }
            })
        }

        function selectScene(sceneId) {
            if (!sceneId) {
                return
            }

            currentSceneId = sceneId

            var summary = null
            for (var i = 0; i < scenes.length; ++i) {
                if (scenes[i] && scenes[i].id === sceneId) {
                    summary = scenes[i]
                    break
                }
            }
            if (summary) {
                currentScene = {
                    id: summary.id,
                    title: summary.title,
                    subtitle: summary.subtitle,
                    sceneType: summary.sceneType,
                    weather: {
                        state: summary.weatherState,
                        label: summary.weatherLabel,
                        hint: summary.subtitle,
                        bandText: summary.title + " · 细节接入中",
                        tone: summary.tone,
                        accent: summary.accent,
                        texture: summary.weatherState
                    },
                    storyLead: summary.subtitle,
                    summary: summary.subtitle,
                    tone: summary.tone,
                    accent: summary.accent,
                    primaryAction: summary.primaryAction,
                    content: {}
                }
                currentWeather = currentScene.weather
                currentSituation = {
                    sceneId: sceneId,
                    weather: currentWeather,
                    summary: "正在整理此场景的今日态势图…",
                    topPriorities: [],
                    prepared: [],
                    habitBasis: summary.subtitle,
                    actionLabel: summary.primaryAction ? summary.primaryAction.label : "推进一步",
                    pressure: summary.weatherLabel
                }
                setLastNotice(summary.subtitle)
            }

            request("/api/scenes/" + sceneId, "GET", null, function(ok, data, message) {
                if (ok && data) {
                    currentScene = data
                }
                if (message) {
                    setLastNotice(message)
                }
            })

            request("/api/weather/current?scene=" + sceneId, "GET", null, function(ok, data, message) {
                if (ok && data) {
                    currentWeather = data
                }
                if (message) {
                    setLastNotice(message)
                }
            })

            request("/api/situation/" + sceneId, "GET", null, function(ok, data, message) {
                if (ok && data) {
                    currentSituation = data
                }
                if (message) {
                    setLastNotice(message)
                }
            })
        }

        function proceed(actionKey) {
            request("/api/action/proceed", "POST", { scene: currentSceneId, action: actionKey }, function(ok, data, message) {
                if (ok && data) {
                    proceedCompleted(data)
                }
                if (message) {
                    setLastNotice(message)
                }
            })
        }

        function requestRiskPreview(actionKey) {
            request("/api/action/risk-preview", "POST", { scene: currentSceneId, action: actionKey }, function(ok, data, message) {
                if (ok && data) {
                    pendingRiskPreview = data
                    riskPreviewCompleted(data)
                }
                if (message) {
                    setLastNotice(message)
                }
            })
        }

        function authorize(actionKey, depthKey) {
            request("/api/action/authorize", "POST", { scene: currentSceneId, action: actionKey, depth: depthKey }, function(ok, data, message) {
                if (ok && data) {
                    authorizationCompleted(data)
                    loadLogs()
                }
                if (message) {
                    setLastNotice(message)
                }
            })
        }

        function loadMemory() {
            request("/api/memory", "GET", null, function(ok, data, message) {
                if (ok && data) {
                    memory = data
                }
                if (message) {
                    setLastNotice(message)
                }
            })
        }

        function loadLogs() {
            request("/api/logs", "GET", null, function(ok, data, message) {
                if (ok && data) {
                    logs = data
                }
                if (message) {
                    setLastNotice(message)
                }
            })
        }
    }

    readonly property var weatherData: backend.currentWeather && backend.currentWeather.state ? backend.currentWeather : startupWeather
    readonly property var sceneData: backend.currentScene && backend.currentScene.id ? backend.currentScene : startupScene
    readonly property var situationData: backend.currentSituation && backend.currentSituation.sceneId ? backend.currentSituation : startupSituation
    readonly property var memoryData: backend.memory && backend.memory.headline ? backend.memory : startupMemory
    readonly property var scenesData: backend.scenes && backend.scenes.length ? backend.scenes : []

    function openScene(sceneId) {
        if (!sceneId) {
            return;
        }
        situationOpen = false
        stormOpen = false
        draftVisible = false
        runtimeDraft = ({})
        runtimeRiskPreview = ({})
        backend.selectScene(sceneId)
    }

    function openCurrentSceneAction() {
        const action = sceneData.primaryAction || ({})
        if (!action.key) {
            return
        }
        backend.proceed(action.key)
    }

    function authorizeCurrentStorm(depthKey) {
        const action = runtimeRiskPreview.actionKey || (sceneData.primaryAction ? sceneData.primaryAction.key : "")
        if (!action) {
            return
        }
        backend.authorize(action, depthKey)
    }

    Component.onCompleted: backend.bootstrap()

    header: WeatherBand {
        weatherData: root.weatherData
        sceneTitle: root.sceneData.title || qsTr("桌面天气机")
        onClicked: {
            root.situationOpen = !root.situationOpen
            root.stormOpen = false
        }
    }

    footer: Rectangle {
        height: 28
        color: Qt.rgba(0.05, 0.08, 0.14, 0.98)
        border.color: Qt.rgba(1, 1, 1, 0.05)

        RowLayout {
            anchors.fill: parent
            anchors.leftMargin: 16
            anchors.rightMargin: 16
            spacing: 12

            Rectangle {
                width: 8
                height: 8
                radius: 4
                color: backend.connected ? "#74D8FF" : "#FF8E8E"
            }

            Text {
                Layout.fillWidth: true
                text: backend.connected
                    ? (backend.lastNotice || qsTr("本地 Go 服务已连接"))
                    : qsTr("本地 Go 服务未连接，请先启动 backend")
                color: "#C8D7EA"
                font.pixelSize: 11
                elide: Text.ElideRight
            }

            Text {
                text: qsTr("日志 %1 条").arg(backend.logs ? backend.logs.length : 0)
                color: "#89A6C9"
                font.pixelSize: 11
            }

            Button {
                visible: !backend.connected
                text: qsTr("重连")
                onClicked: backend.bootstrap()
                background: Rectangle {
                    radius: 10
                    color: Qt.rgba(0.18, 0.27, 0.38, 0.9)
                    border.color: Qt.rgba(1, 1, 1, 0.08)
                }
                contentItem: Text {
                    text: parent.text
                    color: "#F3F8FF"
                    font.pixelSize: 11
                    horizontalAlignment: Text.AlignHCenter
                    verticalAlignment: Text.AlignVCenter
                }
            }
        }
    }

    Rectangle {
        anchors.fill: parent
        z: -3
        gradient: Gradient {
            GradientStop { position: 0.0; color: "#07111E" }
            GradientStop { position: 0.55; color: "#0B1628" }
            GradientStop { position: 1.0; color: "#050B14" }
        }
    }

    Rectangle {
        x: -120
        y: -110
        width: 520
        height: 520
        radius: 260
        z: -2
        color: Qt.rgba(0.16, 0.33, 0.55, 0.16)
    }

    Rectangle {
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        anchors.margins: -80
        width: 420
        height: 420
        radius: 210
        z: -2
        color: Qt.rgba(0.31, 0.17, 0.59, 0.12)
    }

    ColumnLayout {
        anchors.fill: parent
        anchors.leftMargin: 16
        anchors.rightMargin: 16
        anchors.topMargin: 12
        anchors.bottomMargin: 12
        spacing: 12

        SceneSwitcher {
            Layout.fillWidth: true
            scenes: root.scenesData
            currentSceneId: backend.currentSceneId
            backend: backend
            onSceneSelected: root.openScene(sceneId)
        }

        DesktopSceneArea {
            Layout.fillWidth: true
            Layout.fillHeight: true
            sceneData: root.sceneData
            situationData: root.situationData
            memoryData: root.memoryData
            draftVisible: root.draftVisible
            runtimeDraft: root.runtimeDraft
            backend: backend
            onPrimaryActionTriggered: {
                if (actionKey) {
                    backend.proceed(actionKey)
                }
            }
        }
    }

    SituationOverlay {
        expanded: root.situationOpen
        sceneData: root.sceneData
        situationData: root.situationData
        onCloseRequested: root.situationOpen = false
        onProceedRequested: root.openCurrentSceneAction()
    }

    StormPreviewOverlay {
        expanded: root.stormOpen
        stormData: root.runtimeRiskPreview
        onCloseRequested: root.stormOpen = false
        onConfirmRequested: root.authorizeCurrentStorm(depthKey)
    }

    Connections {
        target: backend

        function onProceedCompleted(result) {
            if (!result || !result.mode) {
                return
            }
            if (result.mode === "draft-open") {
                root.draftVisible = true
                root.runtimeDraft = result.draft || (root.sceneData.content ? root.sceneData.content.draft : ({}))
                root.situationOpen = false
                root.stormOpen = false
            } else if (result.mode === "risk-preview") {
                backend.requestRiskPreview(result.riskActionKey || result.actionKey)
            }
        }

        function onRiskPreviewCompleted(result) {
            root.runtimeRiskPreview = result || ({})
            root.stormOpen = true
            root.situationOpen = false
            root.draftVisible = false
        }

        function onAuthorizationCompleted(result) {
            root.stormOpen = false
            root.situationOpen = false
            backend.loadLogs()
        }
    }
}
