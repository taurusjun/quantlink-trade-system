#include "../include/simulator_config.h"
#include <yaml-cpp/yaml.h>
#include <iostream>

namespace hft {
namespace plugin {
namespace simulator {

bool SimulatorConfig::LoadFromYaml(const std::string& config_file) {
    try {
        YAML::Node config = YAML::LoadFile(config_file);

        if (config["mode"]) {
            mode = config["mode"].as<std::string>();
        }

        if (config["account"]) {
            auto account = config["account"];
            if (account["initial_balance"]) {
                initial_balance = account["initial_balance"].as<double>();
            }
            if (account["commission_rate"]) {
                commission_rate = account["commission_rate"].as<double>();
            }
            if (account["margin_rate"]) {
                margin_rate = account["margin_rate"].as<double>();
            }
        }

        if (config["matching"]) {
            auto matching = config["matching"];
            if (matching["accept_delay_ms"]) {
                accept_delay_ms = matching["accept_delay_ms"].as<int>();
            }
            if (matching["fill_delay_ms"]) {
                fill_delay_ms = matching["fill_delay_ms"].as<int>();
            }
            if (matching["slippage_ticks"]) {
                slippage_ticks = matching["slippage_ticks"].as<double>();
            }
        }

        if (config["risk"]) {
            auto risk = config["risk"];
            if (risk["max_position_per_symbol"]) {
                max_position_per_symbol = risk["max_position_per_symbol"].as<int>();
            }
            if (risk["max_daily_loss"]) {
                max_daily_loss = risk["max_daily_loss"].as<double>();
            }
        }

        if (config["persistence"]) {
            auto persistence = config["persistence"];
            if (persistence["data_dir"]) {
                data_dir = persistence["data_dir"].as<std::string>();
            }
            if (persistence["enable"]) {
                enable_persistence = persistence["enable"].as<bool>();
            }
            if (persistence["snapshot_interval_sec"]) {
                snapshot_interval_sec = persistence["snapshot_interval_sec"].as<int>();
            }
        }

        if (config["log"]) {
            auto log = config["log"];
            if (log["level"]) {
                log_level = log["level"].as<std::string>();
            }
            if (log["console"]) {
                log_to_console = log["console"].as<bool>();
            }
        }

        std::string error;
        if (!Validate(&error)) {
            std::cerr << "Configuration validation failed: " << error << std::endl;
            return false;
        }

        return true;
    } catch (const YAML::Exception& e) {
        std::cerr << "Failed to load config file " << config_file << ": " << e.what() << std::endl;
        return false;
    }
}

bool SimulatorConfig::Validate(std::string* error) const {
    if (mode != "immediate" && mode != "market_driven") {
        if (error) *error = "Invalid mode: " + mode;
        return false;
    }

    if (initial_balance <= 0) {
        if (error) *error = "initial_balance must be positive";
        return false;
    }

    if (commission_rate < 0 || commission_rate > 1) {
        if (error) *error = "commission_rate must be in [0, 1]";
        return false;
    }

    if (margin_rate <= 0 || margin_rate > 1) {
        if (error) *error = "margin_rate must be in (0, 1]";
        return false;
    }

    if (accept_delay_ms < 0 || fill_delay_ms < 0) {
        if (error) *error = "delay_ms must be non-negative";
        return false;
    }

    if (max_position_per_symbol <= 0) {
        if (error) *error = "max_position_per_symbol must be positive";
        return false;
    }

    return true;
}

} // namespace simulator
} // namespace plugin
} // namespace hft
