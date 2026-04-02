cmake_minimum_required(VERSION 3.16)
project(cialloclaw VERSION 0.0.0 LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

set(CMAKE_AUTOMOC ON)
set(CMAKE_AUTORCC ON)
set(CMAKE_AUTOUIC ON)

find_package(Qt6 REQUIRED COMPONENTS Core Gui Widgets Network Qml Quick)

qt_standard_project_setup(REQUIRES 6.5)

qt_add_executable(cialloclaw
    src/main.cpp
    src/PetWindow.h
    src/PetWindow.cpp
    src/ChatWindow.h
    src/ChatWindow.cpp
    src/SettingsWindow.h
    src/SettingsWindow.cpp
)

target_link_libraries(cialloclaw PRIVATE
    Qt6::Core
    Qt6::Gui
    Qt6::Widgets
    Qt6::Network
    Qt6::Qml
    Qt6::Quick
)

set_target_properties(cialloclaw PROPERTIES
    WIN32_EXECUTABLE ON
    MACOSX_BUNDLE ON
)

# Copy resources to build directory
add_custom_command(TARGET cialloclaw POST_BUILD
    COMMAND ${CMAKE_COMMAND} -E copy_directory
    ${CMAKE_SOURCE_DIR}/resources
    $<TARGET_FILE_DIR:cialloclaw>/resources
)
