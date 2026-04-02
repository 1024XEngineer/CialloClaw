import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

Item {
    id: root
    property var situationData: ({})

    function cardAccent(index) {
        switch (index) {
        case 0: return "#76D4D0"
        case 1: return "#8E6BFF"
        default: return "#74D8FF"
        }
    }

    Rectangle {
        anchors.fill: parent
        radius: 22
        color: Qt.rgba(0.08, 0.12, 0.19, 0.70)
        border.color: Qt.rgba(1, 1, 1, 0.08)
    }

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 14
        spacing: 8

        RowLayout {
            Layout.fillWidth: true
            Text {
                text: qsTr("待办管家")
                color: "#F4FAFF"
                font.pixelSize: 14
                font.bold: true
            }
            Item { Layout.fillWidth: true }
            Text {
                text: root.situationData.pressure || qsTr("压力云层")
                color: "#8FB0D4"
                font.pixelSize: 10
            }
        }

        Text {
            Layout.fillWidth: true
            text: root.situationData.summary || qsTr("什么任务最该先做，会在这里变成压力前线。")
            color: "#DCE8F5"
            font.pixelSize: 12
            wrapMode: Text.WordWrap
        }

        Repeater {
            model: root.situationData.topPriorities || []

            delegate: Rectangle {
                required property int index
                required property var modelData
                Layout.fillWidth: true
                Layout.preferredHeight: 44
                radius: 13
                color: Qt.rgba(1, 1, 1, 0.035)
                border.color: Qt.rgba(1, 1, 1, 0.05)

                Rectangle {
                    anchors.left: parent.left
                    anchors.top: parent.top
                    anchors.bottom: parent.bottom
                    width: 3
                    radius: 1.5
                    color: root.cardAccent(index)
                }

                ColumnLayout {
                    anchors.fill: parent
                    anchors.leftMargin: 12
                    anchors.rightMargin: 12
                    anchors.topMargin: 7
                    anchors.bottomMargin: 7
                    spacing: 0

                    RowLayout {
                        Layout.fillWidth: true
                        Text {
                            text: (index + 1) + ". " + (modelData.title || "")
                            color: "#FFFFFF"
                            font.pixelSize: 11
                            font.bold: true
                            elide: Text.ElideRight
                        }
                        Item { Layout.fillWidth: true }
                        Text {
                            text: modelData.whyNow || ""
                            color: root.cardAccent(index)
                            font.pixelSize: 10
                            elide: Text.ElideRight
                        }
                    }

                    Text {
                        Layout.fillWidth: true
                        text: modelData.prepared ? qsTr("已准备：%1").arg(modelData.prepared) : ""
                        color: "#AFC2D7"
                        font.pixelSize: 10
                        elide: Text.ElideRight
                    }
                }
            }
        }
    }
}
