cmake_minimum_required(VERSION 2.20.0)
project(WebrtcProxy)

find_package(Threads REQUIRED)
list(APPEND SVC_COMPILE_OPTIONS -Wall)
list(PREPEND PLATFORM_LIBRARIES
        ksuser
        wsock32
        ws2_32
        d3d11 dxgi D3DCompiler
        setupapi
        dwmapi
        userenv
        synchronization.lib)
add_executable(thinkremote-svc ./tools/thinkremote-svc.cpp)
set_target_properties(thinkremote-svc PROPERTIES CXX_STANDARD 17)
target_link_libraries(thinkremote-svc
        ${CMAKE_THREAD_LIBS_INIT}
        wtsapi32
        ${PLATFORM_LIBRARIES})
target_compile_options(thinkremote-svc PRIVATE ${SVC_COMPILE_OPTIONS})