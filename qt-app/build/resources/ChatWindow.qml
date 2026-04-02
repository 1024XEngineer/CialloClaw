import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

Rectangle {
    width: 420
    height: 540
    color: "#f1e2c7"
    border.color: "#1f2430"
    border.width: 2
    radius: 14

    Rectangle {
        id: titleBar
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        height: 54
        radius: 14
        color: "#1f2430"

        Rectangle {
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            height: 14
            color: "#1f2430"
        }

        Text {
            anchors.verticalCenter: parent.verticalCenter
            anchors.left: parent.left
            anchors.leftMargin: 16
            text: chatWindow.title
            color: "#fff7ea"
            font.pixelSize: 18
            font.bold: true
        }

        Rectangle {
            z: 3
            width: 28
            height: 28
            radius: 8
            anchors.verticalCenter: parent.verticalCenter
            anchors.right: parent.right
            anchors.rightMargin: 12
            color: "#ffcfbf"
            border.color: "#fff7ea"
            border.width: 2

            Text {
                anchors.centerIn: parent
                text: "×"
                color: "#1f2430"
                font.pixelSize: 16
                font.bold: true
            }

            MouseArea {
                anchors.fill: parent
                onClicked: chatWindow.hideWindow()
            }
        }

        MouseArea {
            z: 1
            anchors.fill: parent
            acceptedButtons: Qt.LeftButton
            onPressed: chatWindow.startWindowMove()
        }
    }

    ColumnLayout {
        anchors.top: titleBar.bottom
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        anchors.margins: 14
        spacing: 10

        Rectangle {
            Layout.fillWidth: true
            radius: 10
            color: "#fff7ea"
            border.color: "#1f2430"
            border.width: 2
            implicitHeight: 66

            Column {
                anchors.fill: parent
                anchors.margins: 10
                spacing: 4

                Text {
                    text: "未来主对话窗口"
                    font.pixelSize: 16
                    font.bold: true
                    color: "#1f2430"
                }

                Text {
                    text: "当前只做假的消息承接，用于证明桌宠可打开独立窗口。"
                    wrapMode: Text.Wrap
                    font.pixelSize: 11
                    color: "#5a6070"
                }
            }
        }

        Rectangle {
            Layout.fillWidth: true
            Layout.fillHeight: true
            radius: 12
            color: "#fffdf8"
            border.color: "#1f2430"
            border.width: 2

            ListView {
                anchors.fill: parent
                anchors.margins: 10
                spacing: 8
                clip: true
                model: chatWindow.messages

                delegate: Rectangle {
                    required property string modelData
                    width: ListView.view.width
                    radius: 8
                    color: index % 2 === 0 ? "#dfeaff" : "#ffe8d6"
                    border.color: "#1f2430"
                    border.width: 1
                    implicitHeight: bubbleText.implicitHeight + 16

                    Text {
                        id: bubbleText
                        anchors.fill: parent
                        anchors.margins: 8
                        text: modelData
                        wrapMode: Text.Wrap
                        font.pixelSize: 13
                        color: "#1f2430"
                    }
                }
            }
        }

        RowLayout {
            Layout.fillWidth: true
            spacing: 8

            Rectangle {
                Layout.fillWidth: true
                implicitHeight: 42
                radius: 10
                color: "#fff7ea"
                border.color: "#1f2430"
                border.width: 2

                TextField {
                    id: inputField
                    anchors.fill: parent
                    anchors.margins: 6
                    placeholderText: "输入一条假的消息..."
                    background: null
                    color: "#1f2430"
                    font.pixelSize: 13
                }
            }

            Rectangle {
                width: 76
                implicitHeight: 42
                radius: 10
                color: sendArea.pressed ? "#7db5ff" : "#8ac6ff"
                border.color: "#1f2430"
                border.width: 2

                Text {
                    anchors.centerIn: parent
                    text: "发送"
                    color: "#1f2430"
                    font.pixelSize: 13
                    font.bold: true
                }

                MouseArea {
                    id: sendArea
                    anchors.fill: parent
                    property bool pressed: false
                    onPressed: pressed = true
                    onReleased: pressed = false
                    onCanceled: pressed = false
                    onClicked: {
                        chatWindow.sendMessage(inputField.text)
                        inputField.text = ""
                    }
                }
            }
        }
    }
}
