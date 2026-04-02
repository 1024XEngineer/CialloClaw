import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

Item {
    id: root
    implicitHeight: 58

    property var weatherData: ({})
    property string sceneTitle: ""

    signal clicked()

    function accent(state) {
        switch (state) {
        case "sunny": return "#74D8FF"
        case "fog": return "#C8D0DF"
        case "storm": return "#8E6BFF"
        case "shower": return "#76D4D0"
        default: return "#9CB7D8"
        }
    }

    function bgA(state) {
        switch (state) {
        case "sunny": return "#10243A"
        case "fog": return "#121C29"
        case "storm": return "#1A1030"
        case "shower": return "#0E2830"
        default: return "#101A2A"
        }
    }

    function bgB(state) {
        switch (state) {
        case "sunny": return "#122E4C"
        case "fog": return "#172332"
        case "storm": return "#2A1650"
        case "shower": return "#113842"
        default: return "#13213B"
        }
    }

    function bars(state) {
        switch (state) {
        case "sunny": return [6, 10, 8, 12, 6]
        case "fog": return [6, 7, 8, 7, 6]
        case "storm": return [14, 8, 18, 10, 14]
        case "shower": return [5, 10, 15, 10, 5]
        default: return [8, 12, 10, 12, 8]
        }
    }

    Rectangle {
        anchors.fill: parent
        gradient: Gradient {
            GradientStop { position: 0; color: root.bgA(root.weatherData.state || "cloudy") }
            GradientStop { position: 1; color: root.bgB(root.weatherData.state || "cloudy") }
        }
    }

    Rectangle {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        height: 1
        color: Qt.rgba(1, 1, 1, 0.08)
    }

    RowLayout {
        anchors.fill: parent
        anchors.leftMargin: 14
        anchors.rightMargin: 14
        spacing: 12

        Rectangle {
            width: 10
            height: 10
            radius: 5
            color: root.accent(root.weatherData.state || "cloudy")
            opacity: 0.96
            Layout.alignment: Qt.AlignVCenter
        }

        ColumnLayout {
            Layout.fillWidth: true
            spacing: 2

            Text {
                text: (root.weatherData.label || "多云") + " · " + (root.weatherData.tone || "轻压但稳定")
                color: "#F1F7FF"
                font.pixelSize: 15
                font.bold: true
                elide: Text.ElideRight
            }

            Text {
                text: root.weatherData.hint || qsTr("正在连接本地天气机…")
                color: "#D0DFEF"
                font.pixelSize: 12
                elide: Text.ElideRight
            }
        }

        ColumnLayout {
            Layout.alignment: Qt.AlignVCenter
            spacing: 0

            Text {
                text: root.sceneTitle || qsTr("桌面天气机")
                color: "#FFFFFF"
                font.pixelSize: 12
                horizontalAlignment: Text.AlignRight
                Layout.alignment: Qt.AlignRight
            }

            Text {
                text: root.weatherData.bandText || qsTr("默认进入多云状态")
                color: "#C7D8EA"
                font.pixelSize: 11
                horizontalAlignment: Text.AlignRight
                Layout.alignment: Qt.AlignRight
            }
        }

        Row {
            spacing: 4
            Layout.alignment: Qt.AlignVCenter
            Repeater {
                model: root.bars(root.weatherData.state || "cloudy")
                delegate: Rectangle {
                    width: 3
                    height: modelData
                    radius: 1.5
                    color: root.accent(root.weatherData.state || "cloudy")
                    opacity: 0.78
                }
            }
        }

        Rectangle {
            implicitWidth: 150
            implicitHeight: 28
            radius: 14
            color: Qt.rgba(0.10, 0.16, 0.25, 0.56)
            border.color: Qt.rgba(1, 1, 1, 0.08)
            Layout.alignment: Qt.AlignVCenter

            Text {
                anchors.centerIn: parent
                text: qsTr("点击展开今日态势图")
                color: "#F6FAFF"
                font.pixelSize: 11
            }
        }
    }

    MouseArea {
        anchors.fill: parent
        hoverEnabled: true
        cursorShape: Qt.PointingHandCursor
        onClicked: root.clicked()
    }
}
