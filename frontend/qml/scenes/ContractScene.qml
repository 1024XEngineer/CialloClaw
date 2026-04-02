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

    function tint(type) {
        return type === "risk-preview" ? "#8E6BFF" : "#76D8FF"
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
                            text: root.sceneData.title || qsTr("合同审阅")
                            color: "#F6FAFF"
                            font.pixelSize: 20
                            font.bold: true
                        }
                        Item { Layout.fillWidth: true }
                        Rectangle {
                            implicitWidth: 108
                            implicitHeight: 28
                            radius: 14
                            color: Qt.rgba(1, 1, 1, 0.05)
                            Text {
                                anchors.centerIn: parent
                                text: root.content.deadline || qsTr("明天提交")
                                color: "#C8D0DF"
                                font.pixelSize: 11
                                font.bold: true
                            }
                        }
                    }

                    Text {
                        text: root.sceneData.storyLead || qsTr("这份合同明天要提交，但还有 2 处未确认。")
                        color: "#D6E2EF"
                        font.pixelSize: 12
                        wrapMode: Text.WordWrap
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

                            RowLayout {
                                Layout.fillWidth: true
                                Text {
                                    text: qsTr("版本差异")
                                    color: "#FFFFFF"
                                    font.pixelSize: 13
                                    font.bold: true
                                }
                                Item { Layout.fillWidth: true }
                                Text {
                                    text: (root.content.versionLeft || "") + " → " + (root.content.versionRight || "")
                                    color: "#C8D0DF"
                                    font.pixelSize: 11
                                }
                            }

                            Repeater {
                                model: root.content.diffLines || []

                                delegate: Rectangle {
                                    required property var modelData
                                    Layout.fillWidth: true
                                    Layout.preferredHeight: 48
                                    radius: 14
                                    color: Qt.rgba(0.18, 0.22, 0.30, 0.45)
                                    border.color: Qt.rgba(1, 1, 1, 0.05)

                                    RowLayout {
                                        anchors.fill: parent
                                        anchors.margins: 10
                                        spacing: 10

                                        Text {
                                            Layout.fillWidth: true
                                            text: modelData.left || ""
                                            color: "#B8C8D8"
                                            font.pixelSize: 11
                                            wrapMode: Text.WordWrap
                                        }

                                        Text {
                                            text: "→"
                                            color: "#8E6BFF"
                                            font.pixelSize: 16
                                            font.bold: true
                                        }

                                        Text {
                                            Layout.fillWidth: true
                                            text: modelData.right || ""
                                            color: "#F6FAFF"
                                            font.pixelSize: 11
                                            wrapMode: Text.WordWrap
                                        }
                                    }
                                }
                            }
                        }
                    }
                }
            }

            Rectangle {
                Layout.preferredWidth: 360
                Layout.fillHeight: true
                radius: 22
                color: Qt.rgba(0.09, 0.14, 0.22, 0.74)
                border.color: Qt.rgba(1, 1, 1, 0.08)

                ColumnLayout {
                    anchors.fill: parent
                    anchors.margins: 16
                    spacing: 10

                    Text {
                        text: qsTr("风险点与习惯判断")
                        color: "#F6FAFF"
                        font.pixelSize: 14
                        font.bold: true
                    }

                    Text {
                        text: qsTr("你处理合同时通常先看版本差异。")
                        color: "#76D8FF"
                        font.pixelSize: 12
                        font.bold: true
                        wrapMode: Text.WordWrap
                    }

                    Repeater {
                        model: root.content.riskPoints || []

                        delegate: Text {
                            required property var modelData
                            text: "• " + modelData
                            color: "#DCE8F5"
                            font.pixelSize: 11
                            wrapMode: Text.WordWrap
                        }
                    }

                    Rectangle {
                        Layout.fillWidth: true
                        Layout.preferredHeight: 1
                        color: Qt.rgba(1, 1, 1, 0.08)
                    }

                    Text {
                        text: qsTr("系统已准备")
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
                        text: root.sceneData.primaryAction ? root.sceneData.primaryAction.label : qsTr("生成确认稿")
                        onClicked: {
                            const action = root.sceneData.primaryAction || ({})
                            if (action.key) {
                                root.primaryActionTriggered(action.type, action.key)
                            }
                        }
                        background: Rectangle {
                            radius: 16
                            color: root.tint(root.sceneData.primaryAction ? root.sceneData.primaryAction.type : "risk-preview")
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
