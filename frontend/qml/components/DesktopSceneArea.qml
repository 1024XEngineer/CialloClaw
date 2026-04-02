import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

Item {
    id: root

    property var sceneData: ({})
    property var situationData: ({})
    property var memoryData: ({})
    property bool draftVisible: false
    property var runtimeDraft: ({})
    property var backend: null

    signal primaryActionTriggered(string actionType, string actionKey)

    function sceneSourceFor(type) {
        switch (type) {
        case "reading": return "../scenes/ReadingScene.qml"
        case "contract": return "../scenes/ContractScene.qml"
        case "debug": return "../scenes/DebugScene.qml"
        case "daily": return "../scenes/DailyScene.qml"
        case "resource": return "../scenes/ResourceScene.qml"
        default: return ""
        }
    }

    function syncLoadedScene() {
        if (!sceneLoader.item) {
            return
        }
        sceneLoader.item.sceneData = root.sceneData
        sceneLoader.item.situationData = root.situationData
        sceneLoader.item.memoryData = root.memoryData
        sceneLoader.item.draftVisible = root.draftVisible
        sceneLoader.item.runtimeDraft = root.runtimeDraft
    }

    Rectangle {
        anchors.fill: parent
        radius: 28
        color: Qt.rgba(0.05, 0.09, 0.16, 0.82)
        border.color: Qt.rgba(1, 1, 1, 0.08)
    }

    Repeater {
        model: 14
        delegate: Rectangle {
            required property int index
            x: 24 + index * 98
            y: 22
            width: 1
            height: parent.height - 44
            color: Qt.rgba(1, 1, 1, 0.018)
        }
    }

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 16
        spacing: 12

        Rectangle {
            Layout.fillWidth: true
            Layout.preferredHeight: 86
            radius: 22
            color: Qt.rgba(0.10, 0.15, 0.24, 0.72)
            border.color: Qt.rgba(1, 1, 1, 0.08)

            RowLayout {
                anchors.fill: parent
                anchors.margins: 16
                spacing: 16

                ColumnLayout {
                    Layout.fillWidth: true
                    spacing: 3

                    Text {
                        text: root.sceneData.title || qsTr("正在接入场景…")
                        color: "#F6FAFF"
                        font.pixelSize: 18
                        font.bold: true
                    }

                    Text {
                        text: root.sceneData.subtitle || qsTr("场景细节正在连接本地 Go 服务")
                        color: "#C9D8E8"
                        font.pixelSize: 12
                        wrapMode: Text.WordWrap
                    }

                    Text {
                        text: root.sceneData.storyLead || root.sceneData.summary || qsTr("桌面状态会在这里先铺开，再决定下一步。")
                        color: "#A8BDD2"
                        font.pixelSize: 11
                        wrapMode: Text.WordWrap
                    }
                }

                ColumnLayout {
                    Layout.preferredWidth: 220
                    Layout.alignment: Qt.AlignVCenter
                    spacing: 6

                    Rectangle {
                        Layout.fillWidth: true
                        height: 32
                        radius: 16
                        color: Qt.rgba(0.12, 0.18, 0.28, 0.90)
                        border.color: Qt.rgba(1, 1, 1, 0.08)

                        Text {
                            anchors.centerIn: parent
                            text: (root.sceneData.weather && root.sceneData.weather.label) ? root.sceneData.weather.label : qsTr("天气接入中")
                            color: "#F6FAFF"
                            font.pixelSize: 11
                        }
                    }

                    Rectangle {
                        Layout.fillWidth: true
                        height: 32
                        radius: 16
                        color: Qt.rgba(0.12, 0.18, 0.28, 0.90)
                        border.color: Qt.rgba(1, 1, 1, 0.08)

                        Text {
                            anchors.centerIn: parent
                            text: root.sceneData.primaryAction ? root.sceneData.primaryAction.label : qsTr("动作接入中")
                            color: "#76D8FF"
                            font.pixelSize: 11
                            font.bold: true
                        }
                    }
                }
            }
        }

        Rectangle {
            Layout.fillWidth: true
            Layout.fillHeight: true
            radius: 24
            color: Qt.rgba(0.08, 0.12, 0.19, 0.88)
            border.color: Qt.rgba(1, 1, 1, 0.08)

            Loader {
                id: sceneLoader
                anchors.fill: parent
                anchors.margins: 14
                source: root.sceneData && root.sceneData.sceneType ? Qt.resolvedUrl(root.sceneSourceFor(root.sceneData.sceneType)) : ""
                onLoaded: root.syncLoadedScene()
            }

            BusyIndicator {
                visible: !sceneLoader.item
                running: visible
                anchors.centerIn: parent
            }

            Text {
                visible: !sceneLoader.item
                anchors.horizontalCenter: parent.horizontalCenter
                anchors.top: parent.verticalCenter
                anchors.topMargin: 48
                text: root.backend && root.backend.connected ? qsTr("桌面场景正在加载…") : qsTr("等待本地 Go 服务接入…")
                color: "#B2C5D7"
                font.pixelSize: 12
            }
        }

        RowLayout {
            Layout.fillWidth: true
            Layout.preferredHeight: 180
            spacing: 12

            MemoryHintBlock {
                Layout.preferredWidth: 368
                Layout.fillHeight: true
                memoryData: root.memoryData
            }

            TaskPressureView {
                Layout.fillWidth: true
                Layout.fillHeight: true
                situationData: root.situationData
            }
        }
    }

    Connections {
        target: sceneLoader.item
        ignoreUnknownSignals: true

        function onPrimaryActionTriggered(actionType, actionKey) {
            root.primaryActionTriggered(actionType, actionKey)
        }
    }

    onSceneDataChanged: syncLoadedScene()
    onSituationDataChanged: syncLoadedScene()
    onMemoryDataChanged: syncLoadedScene()
    onDraftVisibleChanged: syncLoadedScene()
    onRuntimeDraftChanged: syncLoadedScene()
}
