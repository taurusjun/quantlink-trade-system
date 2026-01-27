#include "ctp_md_plugin.h"
#include <iostream>
#include <signal.h>
#include <unistd.h>
#include <memory>
#include <thread>

using namespace hft::plugin::ctp;

// 全局变量用于信号处理
CTPMDPlugin* g_plugin = nullptr;

// 信号处理函数
void SignalHandler(int signal) {
    std::cout << "\n[Main] Received signal " << signal << " (Ctrl+C)" << std::endl;
    if (g_plugin) {
        g_plugin->Stop();
    }
}

void PrintUsage(const char* program_name) {
    std::cout << R"(
Usage: )" << program_name << R"( [OPTIONS]

CTP Market Data Plugin - Connects to CTP and publishes market data via shared memory

Options:
  -c, --config FILE        Config file path (default: config/ctp/ctp_md.yaml)
  -h, --help              Show this help message

Examples:
  )" << program_name << R"(
  )" << program_name << R"( -c config/ctp/ctp_md.yaml

Configuration:
  Config file contains:
    - CTP front address
    - Instruments to subscribe
    - User credentials (from config/ctp/ctp_md.secret.yaml)
    - Shared memory settings

)" << std::endl;
}

int main(int argc, char* argv[]) {
    std::cout << R"(
╔═══════════════════════════════════════════════════════╗
║         HFT CTP Market Data Plugin v1.0             ║
╚═══════════════════════════════════════════════════════╝
)" << std::endl;

    // 解析命令行参数
    std::string config_file = "config/ctp/ctp_md.yaml";

    for (int i = 1; i < argc; ++i) {
        std::string arg = argv[i];

        if (arg == "-h" || arg == "--help") {
            PrintUsage(argv[0]);
            return 0;
        } else if ((arg == "-c" || arg == "--config") && i + 1 < argc) {
            config_file = argv[++i];
        } else {
            std::cerr << "Unknown option: " << arg << std::endl;
            PrintUsage(argv[0]);
            return 1;
        }
    }

    std::cout << "[Main] Config file: " << config_file << std::endl;
    std::cout << std::endl;

    try {
        // 创建插件实例
        auto plugin = std::make_unique<CTPMDPlugin>();
        g_plugin = plugin.get();

        // 注册信号处理
        signal(SIGINT, SignalHandler);
        signal(SIGTERM, SignalHandler);

        // 初始化插件
        if (!plugin->Initialize(config_file)) {
            std::cerr << "[Main] ❌ Failed to initialize plugin" << std::endl;
            return 1;
        }

        // 启动插件
        if (!plugin->Start()) {
            std::cerr << "[Main] ❌ Failed to start plugin" << std::endl;
            return 1;
        }

        // 运行（阻塞）
        std::cout << "[Main] Plugin running... (Press Ctrl+C to stop)" << std::endl;
        while (plugin->IsRunning()) {
            std::this_thread::sleep_for(std::chrono::seconds(1));
        }

        std::cout << "[Main] Goodbye!" << std::endl;
        return 0;

    } catch (const std::exception& e) {
        std::cerr << "[Main] ❌ Fatal error: " << e.what() << std::endl;
        std::cerr << "\nTroubleshooting:" << std::endl;
        std::cerr << "  1. Check if config file exists: " << config_file << std::endl;
        std::cerr << "  2. Check if credentials are configured in:" << std::endl;
        std::cerr << "     - config/ctp/ctp_md.secret.yaml" << std::endl;
        std::cerr << "  3. Check if CTP SDK is properly installed" << std::endl;
        std::cerr << "  4. Check if shared memory is available" << std::endl;
        return 1;
    }
}
