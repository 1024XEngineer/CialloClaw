import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

Item {
    id: root
    parent: Overlay.overlay
    anchors.fill: parent
    z: 200

    property bool expanded: false
    property var sceneData: ({})
    property var situationData: ({})

    signal closeRequested()
    signal proceedRequested()

    opacity: expanded ? 1 : 0
    enabled: expanded
    visible: opacity > 0.01

    Behavior on opacity { NumberAnimation { duration: 200; easing.type: Easing.OutCubic } }

    Rectangle {
        anchors.fill: parent
        color: Qt.rgba(0.03, 0.06, 0.11, 0.90)
    }

    Rectangle {
        x: -80
        y: -80
        width: 380
        height: 380
        radius: 190
        color: Qt.rgba(0.18, 0.30, 0.49, 0.18)
    }

    Rectangle {
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        anchors.margins: -70
        width: 420
        height: 420
        radius: 210
        color: Qt.rgba(0.13, 0.20, 0.34, 0.20)
    }

    MouseArea {
        anchors.fill: parent
        onClicked: root.closeRequested()
    }

    Rectangle {
        id: panel
        width: Math.min(parent.width - 96, 1240)
        height: Math.min(parent.height - 96, 780)
        radius: 28
        anchors.centerIn: parent
        color: Qt.rgba(0.07, 0.11, 0.18, 0.96)
        border.color: Qt.rgba(1, 1, 1, 0.08)

        MouseArea {
            anchors.fill: parent
            z: -1
            onClicked: mouse.accepted = true
        }

        opacity: expanded ? 1 : 0
        y: expanded ? 0 : 18
        Behavior on opacity { NumberAnimation { duration: 220; easing.type: Easing.OutCubic } }
        Behavior on y { NumberAnimation { duration: 220; easing.type: Easing.OutCubic } }

        ColumnLayout {
            anchors.fill: parent
            anchors.margins: 18
            spacing: 14

            RowLayout {
                Layout.fillWidth: true
                spacing: 12

                ColumnLayout {
                    Layout.fillWidth: true
                    spacing: 4
                    Text {
                        text: qsTr("今日态势图")
                        color: "#F6FAFF"
                        font.pixelSize: 22
                        font.bold: true
                    }
                    Text {
                        text: root.situationData.summary || qsTr("当前天气说明会先铺开最值得处理的三件事。")
                        color: "#C8D8E8"
                        font.pixelSize: 12
                        wrapMode: Text.WordWrap
                    }
                }

                Rectangle {
                    implicitWidth: 132
                    implicitHeight: 34
                    radius: 17
                    color: Qt.rgba(0.12, 0.18, 0.28, 0.88)
                    border.color: Qt.rgba(1, 1, 1, 0.08)
                    Text {
                        anchors.centerIn: parent
                        text: (root.situationData.weather && root.situationData.weather.label) ? root.situationData.weather.label : qsTr("天气")
                        color: "#76D8FF"
                        font.pixelSize: 12
                        font.bold: true
                    }
                }

                Button {
                    text: qsTr("关闭")
                    onClicked: root.closeRequested()
                    background: Rectangle {
                        radius: 16
                        color: Qt.rgba(0.13, 0.19, 0.29, 0.88)
                        border.color: Qt.rgba(1, 1, 1, 0.08)
                    }
                    contentItem: Text {
                        text: parent.text
                        color: "#F5FAFF"
                        font.pixelSize: 12
                        horizontalAlignment: Text.AlignHCenter
                        verticalAlignment: Text.AlignVCenter
                    }
                }
            }

            Rectangle {
                Layout.fillWidth: true
                Layout.preferredHeight: 74
                radius: 20
                color: Qt.rgba(0.09, 0.14, 0.22, 0.84)
                border.color: Qt.rgba(1, 1, 1, 0.08)

                RowLayout {
                    anchors.fill: parent
                    anchors.margins: 14
                    spacing: 14

                    ColumnLayout {
                        Layout.fillWidth: true
                        spacing: 3
                        Text {
                            text: qsTr("当前天气说明")
                            color: "#9DB5CC"
                            font.pixelSize: 11
                        }
                        Text {
                            text: root.situationData.weather && root.situationData.weather.hint ? root.situationData.weather.hint : qsTr("正在整理这个场景的气压变化。")
                            color: "#F6FAFF"
                            font.pixelSize: 13
                            font.bold: true
                            wrapMode: Text.WordWrap
                        }
                    }

                    ColumnLayout {
                        Layout.preferredWidth: 260
                        spacing: 3
                        Text {
                            text: qsTr("按你的习惯")
                            color: "#9DB5CC"
                            font.pixelSize: 11
                        }
                        Text {
                            text: root.situationData.habitBasis || qsTr("我会优先用你的常用顺序来判断。")
                            color: "#DCE8F5"
                            font.pixelSize: 11
                            wrapMode: Text.WordWrap
                        }
                    }
                }
            }

            GridLayout {
                Layout.fillWidth: true
                Layout.fillHeight: true
                columns: 3
                columnSpacing: 12
                rowSpacing: 12

                Repeater {
                    model: root.situationData.topPriorities || []

                    delegate: Rectangle {
                        required property var modelData
                        Layout.fillWidth: true
                        Layout.fillHeight: true
                        radius: 18
                        color: Qt.rgba(1, 1, 1, 0.035)
                        border.color: Qt.rgba(1, 1, 1, 0.06)

                        ColumnLayout {
                            anchors.fill: parent
                            anchors.margins: 14
                            spacing: 8

                            Text {
                                text: modelData.title || ""
                                color: "#FFFFFF"
                                font.pixelSize: 15
                                font.bold: true
                                wrapMode: Text.WordWrap
                            }

                            Text {
                                text: qsTr("为什么现在：%1").arg(modelData.whyNow || "")
                                color: "#76D8FF"
                                font.pixelSize: 11
                                wrapMode: Text.WordWrap
                            }

                            Text {
                                text: qsTr("系统已准备：%1").arg(modelData.prepared || "")
                                color: "#D0DFEF"
                                font.pixelSize: 11
                                wrapMode: Text.WordWrap
                            }

                            Text {
                                text: qsTr("如果不推进：%1").arg(modelData.consequence || "")
                                color: "#9FB3C7"
                                font.pixelSize: 11
                                wrapMode: Text.WordWrap
                            }
                        }
                    }
                }
            }

            Rectangle {
                Layout.fillWidth: true
                Layout.preferredHeight: 76
                radius: 20
                color: Qt.rgba(0.09, 0.14, 0.22, 0.84)
                border.color: Qt.rgba(1, 1, 1, 0.08)

                RowLayout {
                    anchors.fill: parent
                    anchors.margins: 14
                    spacing: 12

                    ColumnLayout {
                        Layout.fillWidth: true
                        spacing: 3
                        Text {
                            text: qsTr("系统已经帮你准备了什么")
                            color: "#9DB5CC"
                            font.pixelSize: 11
                        }
                        Text {
                            text: (root.situationData.prepared || []).join(" · ")
                            color: "#DCE8F5"
                            font.pixelSize: 11
                            wrapMode: Text.WordWrap
                        }
                    }

                    Button {
                        text: qsTr("推进一步")
                        onClicked: root.proceedRequested()
                        background: Rectangle {
                            radius: 18
                            color: "#76D4D0"
                        }
                        contentItem: Text {
                            text: parent.text
                            color: "#07111E"
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

    states: [
        State {
            name: "open"
            when: root.expanded
            PropertyChanges { target: panel; opacity: 1; y: 0 }
        },
        State {
            name: "closed"
            when: !root.expanded
            PropertyChanges { target: panel; opacity: 0; y: 18 }
        }
    ]
}
