#include "ctp_config.h"
#include <yaml-cpp/yaml.h>
#include <iostream>
#include <fstream>

namespace hft {
namespace gateway {

bool CTPMDConfig::LoadFromYaml(const std::string& config_file, const std::string& secret_file) {
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
            if (ctp["app_id"]) app_id = ctp["app_id"].as<std::string>();
            if (ctp["auth_code"]) auth_code = ctp["auth_code"].as<std::string>();

            // 订阅合约列表
            if (ctp["instruments"] && ctp["instruments"].IsSequence()) {
                instruments.clear();
                for (const auto& inst : ctp["instruments"]) {
                    instruments.push_back(inst.as<std::string>());
                }
            }
        }

        // 共享内存配置
        if (config["shm"]) {
            auto shm = config["shm"];
            if (shm["queue_name"]) shm_queue_name = shm["queue_name"].as<std::string>();
            if (shm["queue_size"]) shm_queue_size = shm["queue_size"].as<int>();
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

        // 性能配置
        if (config["performance"]) {
            auto perf = config["performance"];
            if (perf["enable_latency_monitor"])
                enable_latency_monitor = perf["enable_latency_monitor"].as<bool>();
            if (perf["latency_log_interval"])
                latency_log_interval = perf["latency_log_interval"].as<int>();
        }

        // 如果主配置中user_id/password为空，尝试从secret文件加载
        if ((user_id.empty() || password.empty()) && !secret_file.empty()) {
            if (!LoadCredentials(secret_file)) {
                std::cerr << "[Config] Warning: Failed to load credentials from " << secret_file << std::endl;
            }
        }

        // 如果还是为空，尝试默认的secret文件路径
        if (user_id.empty() || password.empty()) {
            std::string default_secret = "config/ctp_md.secret.yaml";
            std::ifstream test(default_secret);
            if (test.good()) {
                test.close();
                LoadCredentials(default_secret);
            }
        }

        return true;

    } catch (const YAML::Exception& e) {
        std::cerr << "[Config] YAML parse error: " << e.what() << std::endl;
        return false;
    } catch (const std::exception& e) {
        std::cerr << "[Config] Error: " << e.what() << std::endl;
        return false;
    }
}

bool CTPMDConfig::LoadCredentials(const std::string& secret_file) {
    try {
        YAML::Node secret = YAML::LoadFile(secret_file);

        if (secret["credentials"]) {
            auto cred = secret["credentials"];
            if (cred["user_id"]) user_id = cred["user_id"].as<std::string>();
            if (cred["password"]) password = cred["password"].as<std::string>();

            std::cout << "[Config] Loaded credentials from " << secret_file << std::endl;
            return true;
        }

        return false;

    } catch (const std::exception& e) {
        std::cerr << "[Config] Failed to load credentials: " << e.what() << std::endl;
        return false;
    }
}

bool CTPMDConfig::Validate(std::string* error_msg) const {
    std::string error;

    // 必填字段检查
    if (front_addr.empty()) {
        error = "front_addr is required";
    } else if (broker_id.empty()) {
        error = "broker_id is required";
    } else if (user_id.empty()) {
        error = "user_id is required";
    } else if (password.empty()) {
        error = "password is required";
    } else if (instruments.empty()) {
        error = "instruments list cannot be empty";
    } else if (shm_queue_name.empty()) {
        error = "shm_queue_name is required";
    } else if (shm_queue_size <= 0) {
        error = "shm_queue_size must be positive";
    }

    if (!error.empty()) {
        if (error_msg) {
            *error_msg = error;
        }
        return false;
    }

    return true;
}

void CTPMDConfig::Print() const {
    std::cout << "\n=== CTP Market Data Gateway Configuration ===" << std::endl;
    std::cout << "CTP Settings:" << std::endl;
    std::cout << "  Front Address: " << front_addr << std::endl;
    std::cout << "  Broker ID: " << broker_id << std::endl;
    std::cout << "  User ID: " << user_id << std::endl;
    std::cout << "  Password: " << (password.empty() ? "(empty)" : "******") << std::endl;
    std::cout << "  App ID: " << app_id << std::endl;
    std::cout << "  Auth Code: " << (auth_code.length() > 4 ?
        auth_code.substr(0, 4) + "..." : auth_code) << std::endl;

    std::cout << "\nInstruments (" << instruments.size() << "):" << std::endl;
    for (size_t i = 0; i < instruments.size() && i < 10; ++i) {
        std::cout << "  - " << instruments[i] << std::endl;
    }
    if (instruments.size() > 10) {
        std::cout << "  ... and " << (instruments.size() - 10) << " more" << std::endl;
    }

    std::cout << "\nShared Memory:" << std::endl;
    std::cout << "  Queue Name: " << shm_queue_name << std::endl;
    std::cout << "  Queue Size: " << shm_queue_size << std::endl;

    std::cout << "\nReconnect:" << std::endl;
    std::cout << "  Interval: " << reconnect_interval_sec << "s" << std::endl;
    std::cout << "  Max Attempts: " << (max_reconnect_attempts < 0 ? "unlimited" :
        std::to_string(max_reconnect_attempts)) << std::endl;

    std::cout << "\nLogging:" << std::endl;
    std::cout << "  Level: " << log_level << std::endl;
    std::cout << "  File: " << log_file << std::endl;
    std::cout << "  Console: " << (log_to_console ? "yes" : "no") << std::endl;

    std::cout << "\nPerformance:" << std::endl;
    std::cout << "  Latency Monitor: " << (enable_latency_monitor ? "enabled" : "disabled") << std::endl;
    std::cout << "  Log Interval: " << latency_log_interval << " messages" << std::endl;
    std::cout << "============================================\n" << std::endl;
}

} // namespace gateway
} // namespace hft
