#pragma once
#include <string>

namespace hft {
namespace plugin {
namespace simulator {

struct SimulatorConfig {
    // Matching mode
    std::string mode = "immediate";  // immediate, market_driven

    // Account configuration
    double initial_balance = 1000000.0;
    double commission_rate = 0.0003;
    double margin_rate = 0.10;

    // Matching configuration
    int accept_delay_ms = 50;
    int fill_delay_ms = 100;
    double slippage_ticks = 1.0;

    // Risk control
    int max_position_per_symbol = 1000;
    double max_daily_loss = 100000.0;

    // Persistence
    std::string data_dir = "data/simulator";
    bool enable_persistence = true;
    int snapshot_interval_sec = 60;

    // Logging
    std::string log_level = "info";
    bool log_to_console = true;

    bool LoadFromYaml(const std::string& config_file);
    bool Validate(std::string* error = nullptr) const;
};

} // namespace simulator
} // namespace plugin
} // namespace hft
