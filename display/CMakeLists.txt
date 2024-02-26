cmake_minimum_required(VERSION 2.20.0)
project(Display)



list(APPEND SVC_COMPILE_OPTIONS -Wall)
add_library(display SHARED ./parsec.cpp ./parsec.h ./export.h)
set_target_properties(display PROPERTIES CXX_STANDARD 17)
target_link_libraries(display setupapi cfgmgr32)
target_compile_options(display PRIVATE ${SVC_COMPILE_OPTIONS})


add_executable(test ./main.c)