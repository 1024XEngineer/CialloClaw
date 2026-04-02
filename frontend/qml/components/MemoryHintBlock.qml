import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

Item {
    id: root
    property var memoryData: ({})

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
                text: qsTr("镜子记忆")
                color: "#F4FAFF"
                font.pixelSize: 14
                font.bold: true
            }
            Item { Layout.fillWidth: true }
            Text {
                text: root.memoryData.confidence ? (qsTr("置信 %1").arg(root.memoryData.confidence)) : ""
                color: "#8FB0D4"
                font.pixelSize: 10
            }
        }

        Text {
            Layout.fillWidth: true
            text: root.memoryData.headline || qsTr("这是按你的习惯判断的")
            color: "#DCE8F5"
            font.pixelSize: 12
            font.bold: true
            wrapMode: Text.WordWrap
        }

        Text {
            Layout.fillWidth: true
            text: root.memoryData.note || qsTr("正在读取你的习惯镜像…")
            color: "#A9C0D9"
            font.pixelSize: 11
            wrapMode: Text.WordWrap
        }

        Repeater {
            model: root.memoryData.habits || []

            delegate: Rectangle {
                required property var modelData
                Layout.fillWidth: true
                Layout.preferredHeight: 36
                radius: 12
                color: Qt.rgba(1, 1, 1, 0.035)
                border.color: Qt.rgba(1, 1, 1, 0.05)

                RowLayout {
                    anchors.fill: parent
                    anchors.margins: 10
                    spacing: 8

                    Rectangle {
                        width: 8
                        height: 8
                        radius: 4
                        color: "#76D8FF"
                        Layout.alignment: Qt.AlignVCenter
                    }

                    ColumnLayout {
                        Layout.fillWidth: true
                        spacing: 0
                        Text {
                            text: modelData.scene || ""
                            color: "#F5FAFF"
                            font.pixelSize: 11
                            font.bold: true
                            elide: Text.ElideRight
                        }
                        Text {
                            Layout.fillWidth: true
                            text: modelData.cue || ""
                            color: "#B4C8DB"
                            font.pixelSize: 10
                            elide: Text.ElideRight
                        }
                    }
                }
            }
        }
    }
}
