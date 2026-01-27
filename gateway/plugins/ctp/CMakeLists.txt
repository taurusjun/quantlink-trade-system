# CTP插件独立编译配置

message(STATUS "Configuring CTP Plugin...")

# 查找yaml-cpp
find_package(yaml-cpp QUIET)
if(NOT yaml-cpp_FOUND)
    message(WARNING "yaml-cpp not found - CTP plugin will not be built")
    message(WARNING "Install with: brew install yaml-cpp")
    return()
endif()

# macOS CTP Framework路径
if(APPLE)
    set(CTP_FRAMEWORK_DIR ${CMAKE_CURRENT_SOURCE_DIR}/../../third_party/ctp)

    if(NOT EXISTS "${CTP_FRAMEWORK_DIR}/thostmduserapi_se.framework")
        message(WARNING "CTP framework not found at ${CTP_FRAMEWORK_DIR}")
        message(WARNING "Expected: ${CTP_FRAMEWORK_DIR}/thostmduserapi_se.framework")
        message(WARNING "CTP plugin will not be built")
        return()
    endif()

    message(STATUS "Found CTP framework: ${CTP_FRAMEWORK_DIR}/thostmduserapi_se.framework")

    # CTP MD Plugin可执行文件（使用新名称避免与legacy版本冲突）
    add_executable(ctp_md_plugin
        src/ctp_config.cpp
        src/ctp_md_plugin.cpp
        src/main_ctp_md.cpp
    )

    # 包含目录
    target_include_directories(ctp_md_plugin PRIVATE
        ${CMAKE_CURRENT_SOURCE_DIR}/include
        ${CMAKE_CURRENT_SOURCE_DIR}/../../include  # 访问通用接口和shm_queue.h
        ${CTP_FRAMEWORK_DIR}/include
        /opt/homebrew/include
    )

    # 链接库
    target_link_libraries(ctp_md_plugin
        ${CTP_FRAMEWORK_DIR}/thostmduserapi_se.framework/Versions/A/thostmduserapi_se
        /opt/homebrew/lib/libyaml-cpp.dylib
        pthread
    )

    # 设置RPATH（运行时库搜索路径）
    set_target_properties(ctp_md_plugin PROPERTIES
        BUILD_WITH_INSTALL_RPATH TRUE
        INSTALL_RPATH "${CTP_FRAMEWORK_DIR}"
    )

    # 安装
    install(TARGETS ctp_md_plugin DESTINATION bin)

    message(STATUS "CTP MD Plugin target added: ctp_md_plugin")

elseif(UNIX AND NOT APPLE)
    # Linux CTP库路径（未来支持）
    message(STATUS "CTP plugin on Linux is not yet supported")
    message(STATUS "Linux support will be added in future releases")
else()
    message(STATUS "CTP plugin is only supported on macOS and Linux")
endif()
