cmake_minimum_required(VERSION 2.20.0)
project(Service)

add_executable(thinkremotesvc ./thinkremote-svc.cpp)
set_target_properties(thinkremotesvc PROPERTIES CXX_STANDARD 17)
target_link_libraries(thinkremotesvc)