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
        return type === "proceed" ? "#9CB7D8" : "#76D8FF"
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
                        text: root.sceneData.title || qsTr("系统资源异常")
                        color: "#F6FAFF"
                        font.pixelSize: 20
                        font.bold: true
                    }

                    Text {
                        text: root.content.assistantHint || qsTr("现在更像轻压力，但还没到强制干预。")
                        color: "#DCE8F5"
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

                            Text {
                                text: qsTr("资源指标")
                                color: "#FFFFFF"
                                font.pixelSize: 13
                                font.bold: true
                            }

                            Repeater {
                                model: root.content.metrics || []

                                delegate: Rectangle {
                                    required property var modelData
                                    Layout.fillWidth: true
                                    Layout.preferredHeight: 44
                                    radius: 14
                                    color: Qt.rgba(0.18, 0.22, 0.30, 0.45)
                                    border.color: Qt.rgba(1, 1, 1, 0.05)

                                    RowLayout {
                                        anchors.fill: parent
                                        anchors.margins: 10
                                        spacing: 10

                                        ColumnLayout {
                                            Layout.preferredWidth: 80
                                            spacing: 0
                                            Text {
                                                text: modelData.name || ""
                                                color: "#FFFFFF"
                                                font.pixelSize: 11
                                                font.bold: true
                                            }
                                            Text {
                                                text: modelData.label || ""
                                                color: "#AFC2D7"
                                                font.pixelSize: 9
                                            }
                                        }

                                        Rectangle {
                                            Layout.fillWidth: true
                                            Layout.preferredHeight: 10
                                            radius: 5
                                            color: Qt.rgba(1, 1, 1, 0.06)

                                            Rectangle {
                                                width: Math.max(8, (modelData.value || 0) * 1.8)
                                                height: parent.height
                                                radius: 5
                                                color: root.tint(root.sceneData.primaryAction ? root.sceneData.primaryAction.type : "proceed")
                                            }
                                        }

                                        Text {
                                            text: (modelData.value || 0) + (modelData.unit || "")
                                            color: "#F6FAFF"
                                            font.pixelSize: 11
                                            font.bold: true
                                        }

                                        Text {
                                            text: modelData.trend || ""
                                            color: "#76D8FF"
                                            font.pixelSize: 10
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
                        text: qsTr("热点与趋势")
                        color: "#F6FAFF"
                        font.pixelSize: 14
                        font.bold: true
                    }

                    Repeater {
                        model: root.content.hotspots || []

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
                        text: qsTr("时间线")
                        color: "#9DB5CC"
                        font.pixelSize: 11
                    }

                    Repeater {
                        model: root.content.timeline || []

                        delegate: Text {
                            required property var modelData
                            text: "• " + modelData
                            color: "#BFD1E2"
                            font.pixelSize: 10
                            wrapMode: Text.WordWrap
                        }
                    }

                    Rectangle {
                        Layout.fillWidth: true
                        Layout.preferredHeight: 1
                        color: Qt.rgba(1, 1, 1, 0.08)
                    }

                    Text {
                        text: root.content.systemNote || qsTr("可先做只读观察，再决定是否干预。")
                        color: "#76D8FF"
                        font.pixelSize: 11
                        wrapMode: Text.WordWrap
                    }

                    Item { Layout.fillHeight: true }

                    Button {
                        Layout.fillWidth: true
                        text: root.sceneData.primaryAction ? root.sceneData.primaryAction.label : qsTr("聚焦异常源")
                        onClicked: {
                            const action = root.sceneData.primaryAction || ({})
                            if (action.key) {
                                root.primaryActionTriggered(action.type, action.key)
                            }
                        }
                        background: Rectangle {
                            radius: 16
                            color: root.tint(root.sceneData.primaryAction ? root.sceneData.primaryAction.type : "proceed")
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
