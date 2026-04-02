import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

Item {
    id: root
    anchors.fill: parent

    property var sceneData: ({})
    property var situationData: ({})
    property var memoryData: ({})
    property bool draftVisible: false
    property var runtimeDraft: ({})

    signal primaryActionTriggered(string actionType, string actionKey)

    property var content: root.sceneData.content || ({})

    function actionTint(type) {
        return type === "proceed" ? "#76D8FF" : "#8E6BFF"
    }

    Rectangle {
        anchors.fill: parent
        radius: 0
        color: "transparent"
    }

    ColumnLayout {
        anchors.fill: parent
        spacing: 14

        RowLayout {
            Layout.fillWidth: true
            Layout.fillHeight: true
            spacing: 14

            Rectangle {
                Layout.fillWidth: true
                Layout.fillHeight: true
                radius: 22
                color: Qt.rgba(0.10, 0.14, 0.23, 0.72)
                border.color: Qt.rgba(1, 1, 1, 0.08)

                ColumnLayout {
                    anchors.fill: parent
                    anchors.margins: 16
                    spacing: 10

                    RowLayout {
                        Layout.fillWidth: true
                        Text {
                            text: root.sceneData.content && root.sceneData.content.articleTitle ? root.sceneData.content.articleTitle : qsTr("正在加载阅读卡片…")
                            color: "#F6FAFF"
                            font.pixelSize: 20
                            font.bold: true
                        }
                        Item { Layout.fillWidth: true }
                        Rectangle {
                            implicitWidth: 96
                            implicitHeight: 28
                            radius: 14
                            color: Qt.rgba(1, 1, 1, 0.05)
                            Text {
                                anchors.centerIn: parent
                                text: root.content.elapsed || qsTr("-- 分钟")
                                color: "#76D8FF"
                                font.pixelSize: 11
                                font.bold: true
                            }
                        }
                    }

                    Text {
                        text: root.sceneData.content && root.sceneData.content.articleSubtitle ? root.sceneData.content.articleSubtitle : qsTr("先看结论，再回头看证据。")
                        color: "#AFC4D7"
                        font.pixelSize: 12
                    }

                    Rectangle {
                        Layout.fillWidth: true
                        Layout.fillHeight: true
                        radius: 18
                        color: Qt.rgba(1, 1, 1, 0.035)
                        border.color: Qt.rgba(1, 1, 1, 0.05)

                        ColumnLayout {
                            anchors.fill: parent
                            anchors.margins: 14
                            spacing: 10

                            Text {
                                text: qsTr("重点提纯")
                                color: "#FFFFFF"
                                font.pixelSize: 13
                                font.bold: true
                            }

                            Repeater {
                                model: root.content.focusPoints || []

                                delegate: Text {
                                    required property var modelData
                                    text: "• " + modelData
                                    color: "#D7E6F5"
                                    font.pixelSize: 11
                                    wrapMode: Text.WordWrap
                                }
                            }

                            Text {
                                visible: !(root.content.focusPoints && root.content.focusPoints.length)
                                text: qsTr("正在等待重点提纯的内容…")
                                color: "#9CB0C3"
                                font.pixelSize: 11
                            }
                        }
                    }

                    RowLayout {
                        Layout.fillWidth: true
                        spacing: 8
                        Repeater {
                            model: root.content.tabs || []

                            delegate: Rectangle {
                                required property var modelData
                                implicitHeight: 28
                                implicitWidth: 120
                                radius: 14
                                color: Qt.rgba(0.18, 0.25, 0.36, 0.70)
                                border.color: Qt.rgba(1, 1, 1, 0.06)

                                Column {
                                    anchors.centerIn: parent
                                    spacing: 0
                                    Text {
                                        text: modelData.title || ""
                                        color: "#F5FAFF"
                                        font.pixelSize: 10
                                        font.bold: true
                                        horizontalAlignment: Text.AlignHCenter
                                    }
                                    Text {
                                        text: modelData.state || ""
                                        color: "#96B1CC"
                                        font.pixelSize: 9
                                        horizontalAlignment: Text.AlignHCenter
                                    }
                                }
                            }
                        }
                    }
                }
            }

            Rectangle {
                Layout.preferredWidth: 340
                Layout.fillHeight: true
                radius: 22
                color: Qt.rgba(0.09, 0.14, 0.22, 0.74)
                border.color: Qt.rgba(1, 1, 1, 0.08)

                ColumnLayout {
                    anchors.fill: parent
                    anchors.margins: 16
                    spacing: 10

                    Text {
                        text: qsTr("系统态势")
                        color: "#F6FAFF"
                        font.pixelSize: 14
                        font.bold: true
                    }

                    Text {
                        text: root.content.assistantHint || qsTr("先提炼重点，不要一次读完。")
                        color: "#DCE8F5"
                        font.pixelSize: 12
                        wrapMode: Text.WordWrap
                    }

                    Rectangle {
                        Layout.fillWidth: true
                        Layout.preferredHeight: 1
                        color: Qt.rgba(1, 1, 1, 0.08)
                    }

                    Text {
                        text: qsTr("已自动准备")
                        color: "#9DB5CC"
                        font.pixelSize: 11
                    }

                    Repeater {
                        model: root.content.prepared || []

                        delegate: Text {
                            required property var modelData
                            text: "• " + modelData
                            color: "#BFD1E2"
                            font.pixelSize: 11
                            wrapMode: Text.WordWrap
                        }
                    }

                    Item { Layout.fillHeight: true }

                    Button {
                        Layout.fillWidth: true
                        text: root.sceneData.primaryAction ? root.sceneData.primaryAction.label : qsTr("提炼重点")
                        onClicked: {
                            const action = root.sceneData.primaryAction || ({})
                            if (action.key) {
                                root.primaryActionTriggered(action.type, action.key)
                            }
                        }
                        background: Rectangle {
                            radius: 16
                            color: root.actionTint(root.sceneData.primaryAction ? root.sceneData.primaryAction.type : "proceed")
                        }
                        contentItem: Text {
                            text: parent.text
                            color: "#08111E"
                            font.pixelSize: 13
                            font.bold: true
                            horizontalAlignment: Text.AlignHCenter
                            verticalAlignment: Text.AlignVCenter
                        }
                    }

                    Text {
                        text: root.sceneData.primaryAction ? root.sceneData.primaryAction.description : ""
                        color: "#9FB3C7"
                        font.pixelSize: 10
                        wrapMode: Text.WordWrap
                    }
                }
            }
        }
    }
}
