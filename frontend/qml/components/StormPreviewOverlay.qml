import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

Item {
    id: root
    parent: Overlay.overlay
    anchors.fill: parent
    z: 220

    property bool expanded: false
    property var stormData: ({})

    signal closeRequested()
    signal confirmRequested(string depthKey)

    readonly property var levelKeys: ["preview", "draft", "execute_once"]

    opacity: expanded ? 1 : 0
    enabled: expanded
    visible: opacity > 0.01
    Behavior on opacity { NumberAnimation { duration: 200; easing.type: Easing.OutCubic } }

    function depthIndexForKey(key) {
        const idx = levelKeys.indexOf(key)
        return idx >= 0 ? idx : 1
    }

    Rectangle {
        anchors.fill: parent
        color: Qt.rgba(0.02, 0.04, 0.08, 0.94)
    }

    Rectangle {
        x: -120
        y: -100
        width: 500
        height: 500
        radius: 250
        color: Qt.rgba(0.36, 0.18, 0.72, 0.18)
    }

    Rectangle {
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        anchors.margins: -100
        width: 560
        height: 560
        radius: 280
        color: Qt.rgba(0.28, 0.12, 0.20, 0.18)
    }

    MouseArea {
        anchors.fill: parent
        onClicked: root.closeRequested()
    }

    Rectangle {
        id: panel
        width: Math.min(parent.width - 72, 1260)
        height: Math.min(parent.height - 72, 820)
        radius: 28
        anchors.centerIn: parent
        color: Qt.rgba(0.07, 0.09, 0.16, 0.97)
        border.color: Qt.rgba(1, 1, 1, 0.08)

        MouseArea {
            anchors.fill: parent
            z: -1
            onClicked: mouse.accepted = true
        }

        opacity: expanded ? 1 : 0
        y: expanded ? 0 : 22
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
                        text: qsTr("雷暴预演")
                        color: "#F6FAFF"
                        font.pixelSize: 22
                        font.bold: true
                    }
                    Text {
                        text: root.stormData.summary || qsTr("系统正在把高风险动作先走一遍，不会直接打断你。")
                        color: "#D0DDF0"
                        font.pixelSize: 12
                        wrapMode: Text.WordWrap
                    }
                }

                Rectangle {
                    implicitWidth: 132
                    implicitHeight: 34
                    radius: 17
                    color: Qt.rgba(0.18, 0.10, 0.26, 0.92)
                    border.color: Qt.rgba(1, 1, 1, 0.08)
                    Text {
                        anchors.centerIn: parent
                        text: root.stormData.riskLevel || qsTr("高压")
                        color: "#D8B7FF"
                        font.pixelSize: 12
                        font.bold: true
                    }
                }

                Button {
                    text: qsTr("关闭")
                    onClicked: root.closeRequested()
                    background: Rectangle {
                        radius: 16
                        color: Qt.rgba(0.14, 0.19, 0.28, 0.88)
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
                Layout.preferredHeight: 72
                radius: 20
                color: Qt.rgba(0.10, 0.13, 0.22, 0.84)
                border.color: Qt.rgba(1, 1, 1, 0.08)

                RowLayout {
                    anchors.fill: parent
                    anchors.margins: 14
                    spacing: 14

                    ColumnLayout {
                        Layout.fillWidth: true
                        spacing: 3
                        Text {
                            text: qsTr("动作名称")
                            color: "#9DB5CC"
                            font.pixelSize: 11
                        }
                        Text {
                            text: root.stormData.actionLabel || qsTr("待确认")
                            color: "#F6FAFF"
                            font.pixelSize: 15
                            font.bold: true
                        }
                    }

                    ColumnLayout {
                        Layout.preferredWidth: 300
                        spacing: 3
                        Text {
                            text: qsTr("将影响的对象")
                            color: "#9DB5CC"
                            font.pixelSize: 11
                        }
                        Text {
                            text: (root.stormData.impacts || []).join(" · ")
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
                    model: [
                        { title: qsTr("可回滚项"), data: root.stormData.rollbacks || [] },
                        { title: qsTr("不可逆项"), data: root.stormData.irreversible || [] },
                        { title: qsTr("授权边界"), data: root.stormData.authorizationLevels || [] }
                    ]

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
                                font.pixelSize: 14
                                font.bold: true
                                wrapMode: Text.WordWrap
                            }

                            Repeater {
                                model: modelData.data || []

                                delegate: Text {
                                    required property var modelData
                                    text: (typeof modelData === "object" && modelData.label) ? ("• " + modelData.label + " — " + (modelData.description || "")) : ("• " + modelData)
                                    color: "#C9D7E8"
                                    font.pixelSize: 11
                                    wrapMode: Text.WordWrap
                                }
                            }
                        }
                    }
                }
            }

            Rectangle {
                Layout.fillWidth: true
                Layout.preferredHeight: 150
                radius: 20
                color: Qt.rgba(0.10, 0.13, 0.22, 0.84)
                border.color: Qt.rgba(1, 1, 1, 0.08)

                ColumnLayout {
                    anchors.fill: parent
                    anchors.margins: 14
                    spacing: 8

                    AuthorizationDepthSlider {
                        id: slider
                        Layout.fillWidth: true
                        depthIndex: root.depthIndexForKey(root.stormData.recommendedDepth || "draft")
                        onDepthIndexChanged: {
                            /* parent reads selectedKey on confirm */
                        }
                    }

                    RowLayout {
                        Layout.fillWidth: true
                        spacing: 10
                        Text {
                            text: qsTr("建议深度：%1").arg(slider.selectedLabel)
                            color: "#76D8FF"
                            font.pixelSize: 11
                            font.bold: true
                        }
                        Item { Layout.fillWidth: true }
                        Button {
                            text: qsTr("确认执行")
                            onClicked: root.confirmRequested(slider.selectedKey)
                            background: Rectangle {
                                radius: 16
                                color: "#D8B7FF"
                            }
                            contentItem: Text {
                                text: parent.text
                                color: "#120C1C"
                                font.pixelSize: 12
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

    states: [
        State {
            name: "open"
            when: root.expanded
            PropertyChanges { target: panel; opacity: 1; y: 0 }
        },
        State {
            name: "closed"
            when: !root.expanded
            PropertyChanges { target: panel; opacity: 0; y: 22 }
        }
    ]
}
