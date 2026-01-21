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

# Counter Gateway可执行文件
set(COUNTER_GATEWAY_SRCS
    src/main_counter.cpp
    src/simulated_counter.cpp
    ${PROTO_SRCS}
    ${GRPC_SRCS}
)

add_executable(counter_gateway ${COUNTER_GATEWAY_SRCS})

target_link_libraries(counter_gateway
    gRPC::grpc++
    gRPC::grpc++_reflection
    Threads::Threads
)

# 安装
install(TARGETS md_gateway md_simulator md_benchmark ors_gateway counter_gateway DESTINATION bin)

# 打印信息
message(STATUS "========================================")
message(STATUS "HFT Gateway POC Configuration")
message(STATUS "========================================")
message(STATUS "Build type: ${CMAKE_BUILD_TYPE}")
message(STATUS "C++ standard: ${CMAKE_CXX_STANDARD}")
message(STATUS "gRPC: Found")
message(STATUS "NATS library: ${NATS_LIB}")
message(STATUS "========================================")
