cmake_minimum_required(VERSION 3.24)

project(CialloClawPrototypeFrontend LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

find_package(Qt6 6.6 REQUIRED COMPONENTS Quick Qml Network)

qt_standard_project_setup(REQUIRES 6.6)

qt_add_executable(cialloclaw_frontend
    WIN32
    main.cpp
    src/backendclient.cpp
    src/backendclient.h
)

qt_add_qml_module(cialloclaw_frontend
    URI CialloClawPrototype
    VERSION 1.0
    QML_FILES
        qml/Main.qml
        qml/components/AppWindow.qml
        qml/components/WeatherBand.qml
        qml/components/SceneSwitcher.qml
        qml/components/DesktopSceneArea.qml
        qml/components/SituationOverlay.qml
        qml/components/StormPreviewOverlay.qml
        qml/components/MemoryHintBlock.qml
        qml/components/TaskPressureView.qml
        qml/components/AuthorizationDepthSlider.qml
        qml/scenes/ReadingScene.qml
        qml/scenes/ContractScene.qml
        qml/scenes/DebugScene.qml
        qml/scenes/DailyScene.qml
        qml/scenes/ResourceScene.qml
)

target_link_libraries(cialloclaw_frontend
    PRIVATE
        Qt6::Quick
        Qt6::Qml
        Qt6::Network
)
