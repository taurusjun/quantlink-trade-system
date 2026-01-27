#include "ctp_md_gateway.h"
#include <iostream>
#include <signal.h>
#include <unistd.h>

using namespace hft::gateway;

// 全局变量用于信号处理
CTPMDGateway* g_gateway = nullptr;

// 信号处理函数
void SignalHandler(int signal) {
    std::cout << "\n[Main] Received signal " << signal << " (Ctrl+C)" << std::endl;
    if (g_gateway) {
        g_gateway->Shutdown();
    }
}

void PrintUsage(const char* program_name) {
    std::cout << R"(
Usage: )" << program_name << R"( [OPTIONS]

CTP Market Data Gateway - Connects to CTP and publishes market data

Options:
  -c, --config FILE        Config file path (default: config/ctp_md.yaml)
  -s, --secret FILE        Secret file path (default: config/ctp_md.secret.yaml)
  -h, --help              Show this help message

Examples:
  )" << program_name << R"(
  )" << program_name << R"( -c config/ctp_md.yaml
  )" << program_name << R"( -c config/ctp_md.yaml -s config/ctp_md.secret.yaml

Configuration:
  Main config file contains CTP front address, instruments to subscribe, etc.
  Secret file contains sensitive credentials (user_id and password).

  If secret file is not specified, will try to load from:
    1. config/ctp_md.secret.yaml (default location)
    2. Credentials in main config file (if present)

)" << std::endl;
}

int main(int argc, char* argv[]) {
    std::cout << R"(
╔═══════════════════════════════════════════════════════╗
║      HFT CTP Market Data Gateway - Production       ║
╚═══════════════════════════════════════════════════════╝
)" << std::endl;

    // 解析命令行参数
    std::string config_file = "config/ctp_md.yaml";
    std::string secret_file = "";

    for (int i = 1; i < argc; ++i) {
        std::string arg = argv[i];

        if (arg == "-h" || arg == "--help") {
            PrintUsage(argv[0]);
            return 0;
        } else if ((arg == "-c" || arg == "--config") && i + 1 < argc) {
            config_file = argv[++i];
        } else if ((arg == "-s" || arg == "--secret") && i + 1 < argc) {
            secret_file = argv[++i];
        } else {
            std::cerr << "Unknown option: " << arg << std::endl;
            PrintUsage(argv[0]);
            return 1;
        }
    }

    std::cout << "[Main] Config file: " << config_file << std::endl;
    if (!secret_file.empty()) {
        std::cout << "[Main] Secret file: " << secret_file << std::endl;
    }
    std::cout << std::endl;

    try {
        // 加载配置
        CTPMDConfig config;
        if (!config.LoadFromYaml(config_file, secret_file)) {
            std::cerr << "[Main] ❌ Failed to load config from " << config_file << std::endl;
            return 1;
        }

        // 验证配置
        std::string error;
        if (!config.Validate(&error)) {
            std::cerr << "[Main] ❌ Invalid config: " << error << std::endl;
            std::cerr << "[Main] Please check your configuration files:" << std::endl;
            std::cerr << "  - " << config_file << std::endl;
            if (!secret_file.empty()) {
                std::cerr << "  - " << secret_file << std::endl;
            } else {
                std::cerr << "  - config/ctp_md.secret.yaml (default)" << std::endl;
            }
            return 1;
        }

        // 注册信号处理
        signal(SIGINT, SignalHandler);
        signal(SIGTERM, SignalHandler);

        // 创建网关
        auto gateway = std::make_unique<CTPMDGateway>(config);
        g_gateway = gateway.get();

        // 运行网关（阻塞）
        gateway->Run();

        std::cout << "[Main] Goodbye!" << std::endl;
        return 0;

    } catch (const std::exception& e) {
        std::cerr << "[Main] ❌ Fatal error: " << e.what() << std::endl;
        std::cerr << "\nTroubleshooting:" << std::endl;
        std::cerr << "  1. Check if config file exists: " << config_file << std::endl;
        std::cerr << "  2. Check if credentials are configured in:" << std::endl;
        std::cerr << "     - config/ctp_md.secret.yaml" << std::endl;
        std::cerr << "  3. Check if CTP SDK is properly installed" << std::endl;
        std::cerr << "  4. Check if shared memory is available" << std::endl;
        return 1;
    }
}
