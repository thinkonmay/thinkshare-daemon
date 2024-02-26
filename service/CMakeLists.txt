cmake_minimum_required(VERSION 2.20.0)
project(Service)

find_package(Threads REQUIRED)
add_executable(thinkremotesvc ./thinkremote-svc.cpp)
set_target_properties(thinkremotesvc PROPERTIES CXX_STANDARD 17)
target_link_libraries(thinkremotesvc ${CMAKE_THREAD_LIBS_INIT} )

set_property(TARGET thinkremotesvc APPEND_STRING PROPERTY LINK_FLAGS " -static")
set_property(TARGET thinkremotesvc APPEND_STRING PROPERTY LINK_FLAGS " -static-libgcc")
set_property(TARGET thinkremotesvc APPEND_STRING PROPERTY LINK_FLAGS " -static-libstdc++")