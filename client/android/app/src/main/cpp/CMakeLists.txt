cmake_minimum_required(VERSION 3.22.1)

project(novpn_tun2proxy_jni)

add_library(tun2proxy SHARED IMPORTED)
set_target_properties(
    tun2proxy PROPERTIES
    IMPORTED_LOCATION "${CMAKE_CURRENT_LIST_DIR}/../jniLibs/${ANDROID_ABI}/libtun2proxy.so"
)

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

target_link_libraries(
    novpn_tun2proxy_jni
    tun2proxy
    ${log-lib}
)
