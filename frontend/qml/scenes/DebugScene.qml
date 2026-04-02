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

                    Text {
                        text: root.sceneData.title || qsTr("报错排查")
                        color: "#F6FAFF"
                        font.pixelSize: 20
                        font.bold: true
                    }

                    Text {
                        text: root.content.errorTitle || qsTr("错误标题正在加载…")
                        color: "#D8C2FF"
                        font.pixelSize: 12
                        font.bold: true
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

                            Text {
                                text: qsTr("人话解释")
                                color: "#FFFFFF"
                                font.pixelSize: 13
                                font.bold: true
                            }

                            Text {
                                text: root.content.humanExplain || qsTr("不是权限拦了，而是路径基准点错了。")
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
                                text: qsTr("排查路径")
                                color: "#9DB5CC"
                                font.pixelSize: 11
                            }

                            Repeater {
                                model: root.content.traceLines || []

                                delegate: Text {
                                    required property var modelData
                                    text: "• " + modelData
                                    color: "#BFD1E2"
                                    font.pixelSize: 11
                                    wrapMode: Text.WordWrap
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
                        text: qsTr("最可能原因")
                        color: "#F6FAFF"
                        font.pixelSize: 14
                        font.bold: true
                    }

                    Repeater {
                        model: root.content.probableCauses || []

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
                        text: qsTr("下一步排查")
                        color: "#9DB5CC"
                        font.pixelSize: 11
                    }

                    Repeater {
                        model: root.content.nextSteps || []

                        delegate: Text {
                            required property var modelData
                            text: "• " + modelData
                            color: "#BFD1E2"
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
                        text: qsTr("按你的习惯")
                        color: "#9DB5CC"
                        font.pixelSize: 11
                    }

                    Repeater {
                        model: root.content.habitChecks || []

                        delegate: Text {
                            required property var modelData
                            text: "• " + modelData
                            color: "#76D8FF"
                            font.pixelSize: 10
                            wrapMode: Text.WordWrap
                        }
                    }

                    Item { Layout.fillHeight: true }

                    Button {
                        Layout.fillWidth: true
                        text: root.sceneData.primaryAction ? root.sceneData.primaryAction.label : qsTr("自动修复建议")
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
                }
            }
        }
    }
}
