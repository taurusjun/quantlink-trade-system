cmake_minimum_required(VERSION 3.15)
project(hft-gateway-poc VERSION 0.1.0 LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

# 编译选项
set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -Wall -Wextra -O2")
set(CMAKE_CXX_FLAGS_DEBUG "${CMAKE_CXX_FLAGS_DEBUG} -g -O0 -DDEBUG")
set(CMAKE_CXX_FLAGS_RELEASE "${CMAKE_CXX_FLAGS_RELEASE} -O3 -DNDEBUG")

# 查找依赖
# 先查找gRPC（包含protobuf），避免重复定义
find_package(gRPC CONFIG REQUIRED)
find_package(Threads REQUIRED)

# 查找protoc和grpc_cpp_plugin可执行文件
find_program(PROTOC_EXECUTABLE protoc REQUIRED)
find_program(GRPC_CPP_PLUGIN grpc_cpp_plugin REQUIRED)

# NATS.c客户端（可选依赖）
find_library(NATS_LIB nats)
if(NATS_LIB)
    message(STATUS "Found NATS library: ${NATS_LIB}")
    add_definitions(-DENABLE_NATS)
    set(HAS_NATS TRUE)
else()
    message(WARNING "NATS library not found - NATS features will be disabled")
    message(WARNING "To enable NATS, run: ./scripts/install_nats_c.sh")
    set(NATS_LIB "")
    set(HAS_NATS FALSE)
endif()

# yaml-cpp（用于CTP配置加载）
find_package(yaml-cpp QUIET)
if(yaml-cpp_FOUND)
    message(STATUS "Found yaml-cpp: ${yaml-cpp_VERSION}")
    set(HAS_YAML_CPP TRUE)
else()
    message(WARNING "yaml-cpp not found - CTP gateway will not be built")
    message(WARNING "To install: brew install yaml-cpp (macOS) or apt install libyaml-cpp-dev (Linux)")
    set(HAS_YAML_CPP FALSE)
endif()

# 生成protobuf和gRPC代码
set(PROTO_PATH "${CMAKE_CURRENT_SOURCE_DIR}/proto")
set(GENERATED_PROTOBUF_PATH "${CMAKE_CURRENT_BINARY_DIR}/generated")
file(MAKE_DIRECTORY ${GENERATED_PROTOBUF_PATH})

set(PROTO_FILES
    "${PROTO_PATH}/common.proto"
    "${PROTO_PATH}/market_data.proto"
    "${PROTO_PATH}/order.proto"
)

# protobuf生成
set(PROTO_SRCS "")
set(PROTO_HDRS "")
set(GRPC_SRCS "")
set(GRPC_HDRS "")

foreach(PROTO_FILE ${PROTO_FILES})
    get_filename_component(PROTO_NAME ${PROTO_FILE} NAME_WE)

    list(APPEND PROTO_SRCS "${GENERATED_PROTOBUF_PATH}/${PROTO_NAME}.pb.cc")
    list(APPEND PROTO_HDRS "${GENERATED_PROTOBUF_PATH}/${PROTO_NAME}.pb.h")
    list(APPEND GRPC_SRCS "${GENERATED_PROTOBUF_PATH}/${PROTO_NAME}.grpc.pb.cc")
    list(APPEND GRPC_HDRS "${GENERATED_PROTOBUF_PATH}/${PROTO_NAME}.grpc.pb.h")

    add_custom_command(
        OUTPUT "${GENERATED_PROTOBUF_PATH}/${PROTO_NAME}.pb.cc"
               "${GENERATED_PROTOBUF_PATH}/${PROTO_NAME}.pb.h"
               "${GENERATED_PROTOBUF_PATH}/${PROTO_NAME}.grpc.pb.cc"
               "${GENERATED_PROTOBUF_PATH}/${PROTO_NAME}.grpc.pb.h"
        COMMAND ${PROTOC_EXECUTABLE}
        ARGS --grpc_out=${GENERATED_PROTOBUF_PATH}
             --cpp_out=${GENERATED_PROTOBUF_PATH}
             -I${PROTO_PATH}
             --plugin=protoc-gen-grpc=${GRPC_CPP_PLUGIN}
             ${PROTO_FILE}
        DEPENDS ${PROTO_FILE}
        COMMENT "Generating protobuf and gRPC C++ files for ${PROTO_NAME}"
    )
endforeach()

# 头文件目录
include_directories(
    ${CMAKE_CURRENT_SOURCE_DIR}/include
    ${GENERATED_PROTOBUF_PATH}
)

# MD Gateway可执行文件
set(MD_GATEWAY_SRCS
    src/main_md.cpp
    src/md_gateway.cpp
    ${PROTO_SRCS}
    ${GRPC_SRCS}
)

add_executable(md_gateway ${MD_GATEWAY_SRCS})

target_link_libraries(md_gateway
    gRPC::grpc++
    gRPC::grpc++_reflection
    ${NATS_LIB}
    Threads::Threads
)

# 模拟器可执行文件
add_executable(md_simulator src/md_simulator.cpp)
target_link_libraries(md_simulator Threads::Threads)

# 性能基准测试工具
add_executable(md_benchmark src/md_benchmark.cpp)
target_link_libraries(md_benchmark Threads::Threads)

# ORS Gateway可执行文件
set(ORS_GATEWAY_SRCS
    src/main_ors.cpp
    src/ors_gateway.cpp
    ${PROTO_SRCS}
    ${GRPC_SRCS}
)

add_executable(ors_gateway ${ORS_GATEWAY_SRCS})

target_link_libraries(ors_gateway
    gRPC::grpc++
    gRPC::grpc++_reflection
    ${NATS_LIB}
    Threads::Threads
)

# CTP Market Data Gateway (仅在macOS且yaml-cpp可用时编译)
if(APPLE AND HAS_YAML_CPP)
    set(CTP_FRAMEWORK_DIR ${CMAKE_CURRENT_SOURCE_DIR}/third_party/ctp)

    if(EXISTS "${CTP_FRAMEWORK_DIR}/thostmduserapi_se.framework")
        message(STATUS "Found CTP framework: ${CTP_FRAMEWORK_DIR}/thostmduserapi_se.framework")

        set(CTP_MD_GATEWAY_SRCS
            src/ctp_config.cpp
            src/ctp_md_gateway.cpp
            src/main_ctp_md.cpp
        )

        add_executable(ctp_md_gateway ${CTP_MD_GATEWAY_SRCS})

        target_include_directories(ctp_md_gateway PRIVATE
            ${CMAKE_CURRENT_SOURCE_DIR}/include
            ${CTP_FRAMEWORK_DIR}/include
            /opt/homebrew/include
        )

        target_link_libraries(ctp_md_gateway
            ${CTP_FRAMEWORK_DIR}/thostmduserapi_se.framework/Versions/A/thostmduserapi_se
            /opt/homebrew/lib/libyaml-cpp.dylib
            Threads::Threads
        )

        # 设置RPATH以便运行时能找到CTP framework
        set_target_properties(ctp_md_gateway PROPERTIES
            BUILD_WITH_INSTALL_RPATH TRUE
            INSTALL_RPATH "${CTP_FRAMEWORK_DIR}"
        )

        set(HAS_CTP_GATEWAY TRUE)
        message(STATUS "CTP MD Gateway will be built")
    else()
        message(WARNING "CTP framework not found at ${CTP_FRAMEWORK_DIR}")
        set(HAS_CTP_GATEWAY FALSE)
    endif()
else()
    set(HAS_CTP_GATEWAY FALSE)
    if(NOT APPLE)
        message(STATUS "CTP MD Gateway only supports macOS - skipping")
    elseif(NOT HAS_YAML_CPP)
        message(STATUS "yaml-cpp not found - CTP MD Gateway skipped")
    endif()
endif()

# 安装
if(HAS_CTP_GATEWAY)
    install(TARGETS md_gateway md_simulator md_benchmark ors_gateway ctp_md_gateway DESTINATION bin)
else()
    install(TARGETS md_gateway md_simulator md_benchmark ors_gateway DESTINATION bin)
endif()

# ==================== 插件编译选项 ====================

option(BUILD_CTP_PLUGIN "Build CTP plugin" ON)
option(BUILD_XTP_PLUGIN "Build XTP plugin" OFF)
option(BUILD_FEMAS_PLUGIN "Build FEMAS plugin" OFF)
option(BUILD_SIMULATOR_PLUGIN "Build Simulator plugin" ON)

# 编译插件
if(BUILD_CTP_PLUGIN)
    message(STATUS "Building CTP plugin...")
    add_subdirectory(plugins/ctp)
endif()

if(BUILD_XTP_PLUGIN)
    message(STATUS "Building XTP plugin...")
    add_subdirectory(plugins/xtp)
endif()

if(BUILD_FEMAS_PLUGIN)
    message(STATUS "Building FEMAS plugin...")
    add_subdirectory(plugins/femas)
endif()

if(BUILD_SIMULATOR_PLUGIN AND HAS_YAML_CPP)
    message(STATUS "Building Simulator plugin...")
    # Simulator plugin does not need subdirectory since it's simple
endif()

# 打印信息
message(STATUS "========================================")
message(STATUS "HFT Gateway POC Configuration")
message(STATUS "========================================")
message(STATUS "Build type: ${CMAKE_BUILD_TYPE}")
message(STATUS "C++ standard: ${CMAKE_CXX_STANDARD}")
message(STATUS "gRPC: Found")
message(STATUS "NATS library: ${NATS_LIB}")
message(STATUS "yaml-cpp: ${yaml-cpp_FOUND}")
message(STATUS "CTP MD Gateway (legacy): ${HAS_CTP_GATEWAY}")
message(STATUS "========================================")
message(STATUS "Plugin Configuration")
message(STATUS "========================================")
message(STATUS "CTP Plugin: ${BUILD_CTP_PLUGIN}")
message(STATUS "XTP Plugin: ${BUILD_XTP_PLUGIN}")
message(STATUS "FEMAS Plugin: ${BUILD_FEMAS_PLUGIN}")
message(STATUS "Simulator Plugin: ${BUILD_SIMULATOR_PLUGIN}")
message(STATUS "========================================")

# ==================== Counter Bridge (通用多券商订单路由网关) ====================
# 从ORS共享内存读取订单，路由到各券商插件（CTP/Simulator/XTP等）

if((BUILD_CTP_PLUGIN OR BUILD_SIMULATOR_PLUGIN) AND APPLE AND HAS_YAML_CPP)
    set(COUNTER_BRIDGE_PLUGINS "")

    if(BUILD_CTP_PLUGIN)
        list(APPEND COUNTER_BRIDGE_PLUGINS "CTP")
    endif()

    if(BUILD_SIMULATOR_PLUGIN)
        list(APPEND COUNTER_BRIDGE_PLUGINS "Simulator")
    endif()

    string(REPLACE ";" ", " COUNTER_BRIDGE_PLUGINS_STR "${COUNTER_BRIDGE_PLUGINS}")
    message(STATUS "Building Counter Bridge with plugins: ${COUNTER_BRIDGE_PLUGINS_STR}")

    set(CTP_FRAMEWORK_DIR ${CMAKE_CURRENT_SOURCE_DIR}/third_party/ctp)

    # Counter Bridge可执行文件
    add_executable(counter_bridge
        src/counter_bridge.cpp
        ${PROTO_SRCS}
        ${GRPC_SRCS}
    )

    # 包含目录
    target_include_directories(counter_bridge PRIVATE
        ${CMAKE_CURRENT_SOURCE_DIR}/include
        ${CMAKE_CURRENT_SOURCE_DIR}/third_party
        ${GENERATED_PROTOBUF_PATH}
        /opt/homebrew/include
    )

    # 添加CTP插件包含目录
    if(BUILD_CTP_PLUGIN)
        target_include_directories(counter_bridge PRIVATE
            ${CMAKE_CURRENT_SOURCE_DIR}/plugins/ctp/include
            ${CTP_FRAMEWORK_DIR}/include
        )
    endif()

    # 添加Simulator插件包含目录
    if(BUILD_SIMULATOR_PLUGIN)
        target_include_directories(counter_bridge PRIVATE
            ${CMAKE_CURRENT_SOURCE_DIR}/plugins/simulator/include
        )
    endif()

    # 基础链接库
    target_link_libraries(counter_bridge
        gRPC::grpc++
        gRPC::grpc++_reflection
        Threads::Threads
        /opt/homebrew/lib/libyaml-cpp.dylib
    )

    # 添加CTP插件
    if(BUILD_CTP_PLUGIN)
        target_compile_definitions(counter_bridge PRIVATE ENABLE_CTP_PLUGIN)
        target_sources(counter_bridge PRIVATE
            plugins/ctp/src/ctp_td_config.cpp
            plugins/ctp/src/ctp_td_plugin.cpp
        )
        target_link_libraries(counter_bridge
            ${CTP_FRAMEWORK_DIR}/thosttraderapi_se.framework/Versions/A/thosttraderapi_se
        )
    endif()

    # 添加Simulator插件
    if(BUILD_SIMULATOR_PLUGIN)
        target_compile_definitions(counter_bridge PRIVATE ENABLE_SIMULATOR_PLUGIN)
        target_sources(counter_bridge PRIVATE
            plugins/simulator/src/simulator_config.cpp
            plugins/simulator/src/simulator_plugin.cpp
            plugins/simulator/src/order_book.cpp
            plugins/simulator/src/matching_engine.cpp
        )
    endif()

    # 设置RPATH
    set_target_properties(counter_bridge PROPERTIES
        BUILD_WITH_INSTALL_RPATH TRUE
        INSTALL_RPATH "${CTP_FRAMEWORK_DIR}"
    )

    # 安装
    install(TARGETS counter_bridge DESTINATION bin)

    message(STATUS "Counter Bridge target added:")
    message(STATUS "  - Reads orders from shared memory (ors_request)")
    message(STATUS "  - Routes to broker plugins: ${COUNTER_BRIDGE_PLUGINS_STR}")
    message(STATUS "  - Writes responses to shared memory (ors_response)")
    message(STATUS "  - Extensible: add more brokers by implementing ITDPlugin")
else()
    message(STATUS "Counter Bridge: Skipped (requires at least one plugin)")
endif()
