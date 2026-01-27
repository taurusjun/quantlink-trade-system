#include "ctp_td_config.h"
#include <yaml-cpp/yaml.h>
#include <iostream>
#include <fstream>

namespace hft {
namespace gateway {

bool CTPTDConfig::LoadFromYaml(const std::string& config_file, const std::string& secret_file) {
    try {
        // 加载主配置文件
        YAML::Node config = YAML::LoadFile(config_file);

        // CTP连接配置
        if (config["ctp"]) {
            auto ctp = config["ctp"];
            if (ctp["front_addr"]) front_addr = ctp["front_addr"].as<std::string>();
            if (ctp["broker_id"]) broker_id = ctp["broker_id"].as<std::string>();
            if (ctp["user_id"]) user_id = ctp["user_id"].as<std::string>();
            if (ctp["password"]) password = ctp["password"].as<std::string>();
            if (ctp["investor_id"]) investor_id = ctp["investor_id"].as<std::string>();
            if (ctp["app_id"]) app_id = ctp["app_id"].as<std::string>();
            if (ctp["auth_code"]) auth_code = ctp["auth_code"].as<std::string>();
            if (ctp["product_info"]) product_info = ctp["product_info"].as<std::string>();
        }

        // 重连配置
        if (config["reconnect"]) {
            auto reconnect = config["reconnect"];
            if (reconnect["interval_sec"]) reconnect_interval_sec = reconnect["interval_sec"].as<int>();
            if (reconnect["max_attempts"]) max_reconnect_attempts = reconnect["max_attempts"].as<int>();
        }

        // 日志配置
        if (config["log"]) {
            auto log = config["log"];
            if (log["level"]) log_level = log["level"].as<std::string>();
            if (log["file"]) log_file = log["file"].as<std::string>();
            if (log["console"]) log_to_console = log["console"].as<bool>();
        }

        // 查询配置
        if (config["query"]) {
            auto query = config["query"];
            if (query["interval_ms"]) query_interval_ms = query["interval_ms"].as<int>();
        }

        // 尝试从密码文件加载敏感信息
        std::string actual_secret_file = secret_file;
        if (actual_secret_file.empty()) {
            // 默认密码文件路径（与config_file同目录）
            size_t last_slash = config_file.find_last_of("/\\");
            if (last_slash != std::string::npos) {
                std::string dir = config_file.substr(0, last_slash + 1);
                actual_secret_file = dir + "ctp_td.secret.yaml";
            } else {
                actual_secret_file = "config/ctp/ctp_td.secret.yaml";
            }
        }

        // 检查密码文件是否存在
        std::ifstream secret_check(actual_secret_file);
        if (secret_check.good()) {
            secret_check.close();
            YAML::Node secret = YAML::LoadFile(actual_secret_file);

            if (secret["credentials"]) {
                auto creds = secret["credentials"];
                if (creds["user_id"]) user_id = creds["user_id"].as<std::string>();
                if (creds["password"]) password = creds["password"].as<std::string>();
                if (creds["investor_id"]) investor_id = creds["investor_id"].as<std::string>();
            }

            std::cout << "[CTPTDConfig] Loaded credentials from " << actual_secret_file << std::endl;
        } else {
            std::cout << "[CTPTDConfig] No secret file found at " << actual_secret_file
                      << ", using credentials from main config" << std::endl;
        }

        // 如果investor_id为空，默认使用user_id
        if (investor_id.empty()) {
            investor_id = user_id;
        }

        return true;

    } catch (const YAML::Exception& e) {
        std::cerr << "[CTPTDConfig] YAML parsing error: " << e.what() << std::endl;
        return false;
    } catch (const std::exception& e) {
        std::cerr << "[CTPTDConfig] Error loading config: " << e.what() << std::endl;
        return false;
    }
}

bool CTPTDConfig::Validate(std::string* error) const {
    // 验证必填字段
    if (front_addr.empty()) {
        if (error) *error = "front_addr is required";
        return false;
    }

    if (broker_id.empty()) {
        if (error) *error = "broker_id is required";
        return false;
    }

    if (user_id.empty()) {
        if (error) *error = "user_id is required";
        return false;
    }

    if (password.empty()) {
        if (error) *error = "password is required";
        return false;
    }

    if (investor_id.empty()) {
        if (error) *error = "investor_id is required";
        return false;
    }

    // 验证数值范围
    if (reconnect_interval_sec < 1) {
        if (error) *error = "reconnect_interval_sec must be >= 1";
        return false;
    }

    if (query_interval_ms < 100) {
        if (error) *error = "query_interval_ms must be >= 100";
        return false;
    }

    return true;
}

void CTPTDConfig::Print() const {
    std::cout << "\n========================================" << std::endl;
    std::cout << "CTP Trading Configuration" << std::endl;
    std::cout << "========================================" << std::endl;
    std::cout << "Front Address: " << front_addr << std::endl;
    std::cout << "Broker ID: " << broker_id << std::endl;
    std::cout << "User ID: " << user_id << std::endl;
    std::cout << "Password: " << (password.empty() ? "(empty)" : "********") << std::endl;
    std::cout << "Investor ID: " << investor_id << std::endl;

    if (!app_id.empty()) {
        std::cout << "App ID: " << app_id << std::endl;
    }
    if (!auth_code.empty()) {
        std::cout << "Auth Code: " << (auth_code.length() > 4 ?
                     auth_code.substr(0, 4) + "..." : "***") << std::endl;
    }
    if (!product_info.empty()) {
        std::cout << "Product Info: " << product_info << std::endl;
    }

    std::cout << "\nReconnect Configuration:" << std::endl;
    std::cout << "  Interval: " << reconnect_interval_sec << " seconds" << std::endl;
    std::cout << "  Max Attempts: " << (max_reconnect_attempts < 0 ? "unlimited" :
                                       std::to_string(max_reconnect_attempts)) << std::endl;

    std::cout << "\nLog Configuration:" << std::endl;
    std::cout << "  Level: " << log_level << std::endl;
    std::cout << "  File: " << (log_file.empty() ? "(none)" : log_file) << std::endl;
    std::cout << "  Console: " << (log_to_console ? "enabled" : "disabled") << std::endl;

    std::cout << "\nQuery Configuration:" << std::endl;
    std::cout << "  Interval: " << query_interval_ms << " ms" << std::endl;
    std::cout << "========================================\n" << std::endl;
}

} // namespace gateway
} // namespace hft
