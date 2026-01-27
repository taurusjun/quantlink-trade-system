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

    # ==================== CTP Trading Plugin ====================

    if(NOT EXISTS "${CTP_FRAMEWORK_DIR}/thosttraderapi_se.framework")
        message(WARNING "CTP trader framework not found at ${CTP_FRAMEWORK_DIR}")
        message(WARNING "Expected: ${CTP_FRAMEWORK_DIR}/thosttraderapi_se.framework")
        message(WARNING "CTP TD plugin will not be built")
    else()
        message(STATUS "Found CTP trader framework: ${CTP_FRAMEWORK_DIR}/thosttraderapi_se.framework")

        # CTP TD Plugin可执行文件
        add_executable(ctp_td_plugin
            src/ctp_td_config.cpp
            src/ctp_td_plugin.cpp
            src/main_ctp_td.cpp
        )

        # 包含目录
        target_include_directories(ctp_td_plugin PRIVATE
            ${CMAKE_CURRENT_SOURCE_DIR}/include
            ${CMAKE_CURRENT_SOURCE_DIR}/../../include  # 访问通用接口
            ${CTP_FRAMEWORK_DIR}/include
            /opt/homebrew/include
        )

        # 链接库
        target_link_libraries(ctp_td_plugin
            ${CTP_FRAMEWORK_DIR}/thosttraderapi_se.framework/Versions/A/thosttraderapi_se
            /opt/homebrew/lib/libyaml-cpp.dylib
            pthread
        )

        # 设置RPATH（运行时库搜索路径）
        set_target_properties(ctp_td_plugin PROPERTIES
            BUILD_WITH_INSTALL_RPATH TRUE
            INSTALL_RPATH "${CTP_FRAMEWORK_DIR}"
        )

        # 安装
        install(TARGETS ctp_td_plugin DESTINATION bin)

        message(STATUS "CTP TD Plugin target added: ctp_td_plugin")

        # ==================== Integrated Test (MD + TD) ====================

        add_executable(ctp_integrated_test
            src/ctp_config.cpp
            src/ctp_md_plugin.cpp
            src/ctp_td_config.cpp
            src/ctp_td_plugin.cpp
            src/main_integrated_test.cpp
        )

        # 包含目录
        target_include_directories(ctp_integrated_test PRIVATE
            ${CMAKE_CURRENT_SOURCE_DIR}/include
            ${CMAKE_CURRENT_SOURCE_DIR}/../../include  # 访问通用接口和shm_queue.h
            ${CTP_FRAMEWORK_DIR}/include
            /opt/homebrew/include
        )

        # 链接库
        target_link_libraries(ctp_integrated_test
            ${CTP_FRAMEWORK_DIR}/thostmduserapi_se.framework/Versions/A/thostmduserapi_se
            ${CTP_FRAMEWORK_DIR}/thosttraderapi_se.framework/Versions/A/thosttraderapi_se
            /opt/homebrew/lib/libyaml-cpp.dylib
            pthread
        )

        # 设置RPATH
        set_target_properties(ctp_integrated_test PROPERTIES
            BUILD_WITH_INSTALL_RPATH TRUE
            INSTALL_RPATH "${CTP_FRAMEWORK_DIR}"
        )

        # 安装
        install(TARGETS ctp_integrated_test DESTINATION bin)

        message(STATUS "CTP Integrated Test target added: ctp_integrated_test")

        # ==================== Simple Integrated Test (TD only with manual MD) ====================

        add_executable(ctp_integrated_simple
            src/ctp_td_config.cpp
            src/ctp_td_plugin.cpp
            src/main_integrated_simple.cpp
        )

        # 包含目录
        target_include_directories(ctp_integrated_simple PRIVATE
            ${CMAKE_CURRENT_SOURCE_DIR}/include
            ${CMAKE_CURRENT_SOURCE_DIR}/../../include
            ${CTP_FRAMEWORK_DIR}/include
            /opt/homebrew/include
        )

        # 链接库
        target_link_libraries(ctp_integrated_simple
            ${CTP_FRAMEWORK_DIR}/thosttraderapi_se.framework/Versions/A/thosttraderapi_se
            /opt/homebrew/lib/libyaml-cpp.dylib
            pthread
        )

        # 设置RPATH
        set_target_properties(ctp_integrated_simple PROPERTIES
            BUILD_WITH_INSTALL_RPATH TRUE
            INSTALL_RPATH "${CTP_FRAMEWORK_DIR}"
        )

        # 安装
        install(TARGETS ctp_integrated_simple DESTINATION bin)

        message(STATUS "CTP Simple Integrated Test target added: ctp_integrated_simple")

        # ==================== Market Order Test (Automated) ====================

        add_executable(ctp_market_order_test
            src/ctp_td_config.cpp
            src/ctp_td_plugin.cpp
            src/main_market_order_test.cpp
        )

        # 包含目录
        target_include_directories(ctp_market_order_test PRIVATE
            ${CMAKE_CURRENT_SOURCE_DIR}/include
            ${CMAKE_CURRENT_SOURCE_DIR}/../../include
            ${CTP_FRAMEWORK_DIR}/include
            /opt/homebrew/include
        )

        # 链接库
        target_link_libraries(ctp_market_order_test
            ${CTP_FRAMEWORK_DIR}/thosttraderapi_se.framework/Versions/A/thosttraderapi_se
            /opt/homebrew/lib/libyaml-cpp.dylib
            pthread
        )

        # 设置RPATH
        set_target_properties(ctp_market_order_test PROPERTIES
            BUILD_WITH_INSTALL_RPATH TRUE
            INSTALL_RPATH "${CTP_FRAMEWORK_DIR}"
        )

        # 安装
        install(TARGETS ctp_market_order_test DESTINATION bin)

        message(STATUS "CTP Market Order Test target added: ctp_market_order_test")

        # ==================== Query and Close Position Tool ====================

        add_executable(ctp_query_and_close
            src/ctp_td_config.cpp
            src/ctp_td_plugin.cpp
            test/query_and_close.cpp
        )

        # 包含目录
        target_include_directories(ctp_query_and_close PRIVATE
            ${CMAKE_CURRENT_SOURCE_DIR}/include
            ${CMAKE_CURRENT_SOURCE_DIR}/../../include
            ${CTP_FRAMEWORK_DIR}/include
            /opt/homebrew/include
        )

        # 链接库
        target_link_libraries(ctp_query_and_close
            ${CTP_FRAMEWORK_DIR}/thosttraderapi_se.framework/Versions/A/thosttraderapi_se
            /opt/homebrew/lib/libyaml-cpp.dylib
            pthread
        )

        # 设置RPATH
        set_target_properties(ctp_query_and_close PROPERTIES
            BUILD_WITH_INSTALL_RPATH TRUE
            INSTALL_RPATH "${CTP_FRAMEWORK_DIR}"
        )

        # 安装
        install(TARGETS ctp_query_and_close DESTINATION bin)

        message(STATUS "CTP Query and Close Position Tool target added: ctp_query_and_close")

        # ==================== Simple Close Position Tool ====================

        add_executable(ctp_close_simple
            src/ctp_td_config.cpp
            src/ctp_td_plugin.cpp
            test/close_position_simple.cpp
        )

        # 包含目录
        target_include_directories(ctp_close_simple PRIVATE
            ${CMAKE_CURRENT_SOURCE_DIR}/include
            ${CMAKE_CURRENT_SOURCE_DIR}/../../include
            ${CTP_FRAMEWORK_DIR}/include
            /opt/homebrew/include
        )

        # 链接库
        target_link_libraries(ctp_close_simple
            ${CTP_FRAMEWORK_DIR}/thosttraderapi_se.framework/Versions/A/thosttraderapi_se
            /opt/homebrew/lib/libyaml-cpp.dylib
            pthread
        )

        # 设置RPATH
        set_target_properties(ctp_close_simple PROPERTIES
            BUILD_WITH_INSTALL_RPATH TRUE
            INSTALL_RPATH "${CTP_FRAMEWORK_DIR}"
        )

        # 安装
        install(TARGETS ctp_close_simple DESTINATION bin)

        message(STATUS "CTP Simple Close Position Tool target added: ctp_close_simple")

        # ==================== Auto Close Position Tool ====================

        add_executable(ctp_close_auto
            src/ctp_td_config.cpp
            src/ctp_td_plugin.cpp
            test/close_position_auto.cpp
        )

        # 包含目录
        target_include_directories(ctp_close_auto PRIVATE
            ${CMAKE_CURRENT_SOURCE_DIR}/include
            ${CMAKE_CURRENT_SOURCE_DIR}/../../include
            ${CTP_FRAMEWORK_DIR}/include
            /opt/homebrew/include
        )

        # 链接库
        target_link_libraries(ctp_close_auto
            ${CTP_FRAMEWORK_DIR}/thosttraderapi_se.framework/Versions/A/thosttraderapi_se
            /opt/homebrew/lib/libyaml-cpp.dylib
            pthread
        )

        # 设置RPATH
        set_target_properties(ctp_close_auto PROPERTIES
            BUILD_WITH_INSTALL_RPATH TRUE
            INSTALL_RPATH "${CTP_FRAMEWORK_DIR}"
        )

        # 安装
        install(TARGETS ctp_close_auto DESTINATION bin)

        message(STATUS "CTP Auto Close Position Tool target added: ctp_close_auto")
    endif()

elseif(UNIX AND NOT APPLE)
    # Linux CTP库路径（未来支持）
    message(STATUS "CTP plugin on Linux is not yet supported")
    message(STATUS "Linux support will be added in future releases")
else()
    message(STATUS "CTP plugin is only supported on macOS and Linux")
endif()
