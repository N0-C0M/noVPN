cmake_minimum_required(VERSION 3.22.1)

project(novpn_tun2proxy_jni)

add_library(
    novpn_tun2proxy_jni
    SHARED
    tun2proxy_jni.cpp
)

target_include_directories(
    novpn_tun2proxy_jni
    PRIVATE
    "${CMAKE_CURRENT_LIST_DIR}/include"
)

find_library(log-lib log)
find_library(dl-lib dl)

target_link_libraries(
    novpn_tun2proxy_jni
    ${log-lib}
    ${dl-lib}
)
