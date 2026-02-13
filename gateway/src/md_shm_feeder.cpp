// md_shm_feeder — Market data feeder to SysV MWMR shared memory
//
// Writes MarketUpdateNew (816 bytes) to SysV MWMR queue at key 0x1001,
// which the Go trader reads via Connector.pollMD().
//
// Two modes:
//   1. CTP mode:       md_shm_feeder ctp:/path/to/ctp_md.yaml
//   2. Simulator mode:  md_shm_feeder simulator:ag2506,ag2512 [--rate 2]
//
// Architecture:
//   CTP/Simulator → md_shm_feeder → [SysV MWMR SHM, key=0x1001] → Go trader
//
// This replaces the old md_gateway → NATS → golang path with a direct SHM path.

#include <iostream>
#include <memory>
#include <thread>
#include <atomic>
#include <signal.h>
#include <cstring>
#include <vector>
#include <string>
#include <sstream>
#include <chrono>
#include <random>
#include <cmath>

#include "hftbase_shm.h"
#include "hftbase_md_types.h"

// CTP plugin (if enabled)
#if defined(ENABLE_CTP_MD)
#include "ctp_config.h"
#include "ThostFtdcMdApi.h"
#endif

using namespace hftbase_compat;
using namespace illuminati::md;

using MDQueue = MWMRQueue<MarketUpdateNew>;

// ============================================================
// Configuration
// ============================================================
struct MDFeederConfig {
    int md_shm_key     = 0x1001; // must match Go trader.tbsrc.yaml md_shm_key
    int md_queue_size   = 65536; // must match Go trader.tbsrc.yaml md_queue_size
};

// ============================================================
// Global variables
// ============================================================
static std::atomic<bool> g_running{true};
static MDQueue* g_md_queue = nullptr;
static std::atomic<uint64_t> g_md_count{0};

void SignalHandler(int signal) {
    std::cout << "\n[MDFeeder] Received signal " << signal << ", shutting down..." << std::endl;
    g_running = false;
}

// ============================================================
// Exchange name mapping
// ============================================================
unsigned char ExchangeNameFromString(const std::string& exchange) {
    if (exchange == "SHFE")  return CHINA_SHFE;
    if (exchange == "CFFEX") return CHINA_CFFEX;
    if (exchange == "CZCE" || exchange == "ZCE") return CHINA_ZCE;
    if (exchange == "DCE")   return CHINA_DCE;
    if (exchange == "GFEX")  return CHINA_GFEX;
    return EXCHANGE_UNKNOWN;
}

// Guess exchange from symbol prefix
// C++ source: tbsrc uses exchange codes per instrument config
unsigned char GuessExchangeFromSymbol(const std::string& symbol) {
    if (symbol.empty()) return CHINA_SHFE;
    // Common SHFE products: ag, au, cu, al, zn, pb, ni, sn, rb, hc, bu, ru, sp, ss, wr, fu
    // Common CFFEX products: IF, IH, IC, IM, T, TF, TS
    // Common DCE products: m, y, a, b, p, c, cs, l, v, pp, j, jm, jd, i, eg, eb, pg, rr, lh
    // Common CZCE products: SR, CF, TA, MA, OI, RM, FG, ZC, SF, SM, AP, CJ, PK, SA, PF, SH, UR
    // Common GFEX products: si, lc

    char c0 = symbol[0];
    if (c0 >= 'A' && c0 <= 'Z') {
        // Uppercase prefix — CFFEX or CZCE
        if (symbol.substr(0, 2) == "IF" || symbol.substr(0, 2) == "IH" ||
            symbol.substr(0, 2) == "IC" || symbol.substr(0, 2) == "IM" ||
            symbol[0] == 'T')
            return CHINA_CFFEX;
        return CHINA_ZCE;
    }
    // Lowercase prefix
    if (symbol.substr(0, 2) == "si" || symbol.substr(0, 2) == "lc")
        return CHINA_GFEX;
    if (c0 == 'm' || c0 == 'y' || c0 == 'a' || c0 == 'b' || c0 == 'p' ||
        c0 == 'c' || c0 == 'l' || c0 == 'v' || c0 == 'j' || c0 == 'i')
        return CHINA_DCE;
    // Default: SHFE
    return CHINA_SHFE;
}

// ============================================================
// Simulator mode — generates simulated MarketUpdateNew
// ============================================================
struct SimulatedInstrument {
    std::string symbol;
    unsigned char exchange;
    double mid_price;
    double tick_size;
    double volatility;  // per-tick volatility
    int base_qty;
};

void RunSimulator(const std::vector<std::string>& symbols, int rate_hz) {
    std::cout << "[Simulator] Starting market data simulator" << std::endl;
    std::cout << "[Simulator] Symbols: ";
    for (size_t i = 0; i < symbols.size(); i++) {
        if (i > 0) std::cout << ", ";
        std::cout << symbols[i];
    }
    std::cout << std::endl;
    std::cout << "[Simulator] Rate: " << rate_hz << " ticks/sec per symbol" << std::endl;

    // Initialize instruments with reasonable default prices
    std::vector<SimulatedInstrument> instruments;
    for (const auto& sym : symbols) {
        SimulatedInstrument inst;
        inst.symbol = sym;
        inst.exchange = GuessExchangeFromSymbol(sym);

        // Set default prices based on product
        if (sym.find("ag") == 0) {
            inst.mid_price = 7800.0;
            inst.tick_size = 1.0;
            inst.base_qty = 5;
        } else if (sym.find("au") == 0) {
            inst.mid_price = 650.0;
            inst.tick_size = 0.02;
            inst.base_qty = 5;
        } else if (sym.find("cu") == 0) {
            inst.mid_price = 72000.0;
            inst.tick_size = 10.0;
            inst.base_qty = 5;
        } else if (sym.find("rb") == 0) {
            inst.mid_price = 3500.0;
            inst.tick_size = 1.0;
            inst.base_qty = 10;
        } else {
            inst.mid_price = 5000.0;
            inst.tick_size = 1.0;
            inst.base_qty = 5;
        }
        inst.volatility = inst.tick_size * 2.0;

        instruments.push_back(inst);
        std::cout << "[Simulator] " << sym << ": mid=" << inst.mid_price
                  << " tick=" << inst.tick_size
                  << " exchange=" << (int)inst.exchange << std::endl;
    }

    // Random engine — correlated price generation for pair trading
    // All instruments of the same product share a common price shock (rho=0.95)
    // plus a small idiosyncratic component, simulating same-commodity different-expiry
    std::mt19937 rng(42);
    std::normal_distribution<double> price_move(0.0, 1.0);
    std::uniform_int_distribution<int> qty_jitter(-3, 10);
    const double correlation = 0.95;  // inter-contract correlation
    const double idio_scale = std::sqrt(1.0 - correlation * correlation);

    auto sleep_us = std::chrono::microseconds(1000000 / rate_hz);
    uint64_t seq = 1;

    while (g_running.load()) {
        // Generate common shock shared by all instruments this tick
        double common_shock = price_move(rng);

        for (auto& inst : instruments) {
            if (!g_running.load()) break;

            // Correlated random walk: rho * common + sqrt(1-rho^2) * idiosyncratic
            double idio_shock = price_move(rng);
            double move = (correlation * common_shock + idio_scale * idio_shock) * inst.volatility;
            inst.mid_price += move;
            // Round to tick size
            inst.mid_price = std::round(inst.mid_price / inst.tick_size) * inst.tick_size;

            // Build MarketUpdateNew
            MarketUpdateNew md;
            std::memset(&md, 0, sizeof(md));

            // Header
            auto now = std::chrono::system_clock::now();
            uint64_t now_ns = std::chrono::duration_cast<std::chrono::nanoseconds>(
                now.time_since_epoch()).count();
            md.m_exchTS = now_ns;
            md.m_timestamp = now_ns;
            md.m_seqnum = seq++;
            std::strncpy(md.m_symbol, inst.symbol.c_str(),
                         sizeof(md.m_symbol) - 1);
            md.m_exchangeName = inst.exchange;

            // Build 5-level order book (CTP gives 5 levels)
            int valid_levels = 5;
            md.m_validBids = valid_levels;
            md.m_validAsks = valid_levels;

            double spread = inst.tick_size;
            double best_bid = inst.mid_price - spread / 2.0;
            double best_ask = inst.mid_price + spread / 2.0;

            // Round to tick
            best_bid = std::floor(best_bid / inst.tick_size) * inst.tick_size;
            best_ask = std::ceil(best_ask / inst.tick_size) * inst.tick_size;
            if (best_ask <= best_bid) best_ask = best_bid + inst.tick_size;

            for (int i = 0; i < valid_levels; i++) {
                md.m_bidUpdates[i].price = best_bid - i * inst.tick_size;
                md.m_bidUpdates[i].quantity = std::max(1, inst.base_qty + qty_jitter(rng));
                md.m_bidUpdates[i].orderCount = 1;

                md.m_askUpdates[i].price = best_ask + i * inst.tick_size;
                md.m_askUpdates[i].quantity = std::max(1, inst.base_qty + qty_jitter(rng));
                md.m_askUpdates[i].orderCount = 1;
            }

            // Last traded price (near mid)
            md.m_lastTradedPrice = (std::uniform_int_distribution<>(0, 1)(rng) == 0)
                ? md.m_bidUpdates[0].price : md.m_askUpdates[0].price;
            md.m_lastTradedQuantity = std::max(1, inst.base_qty / 2 + qty_jitter(rng));

            md.m_feedType = FEED_SNAPSHOT;
            md.m_updateType = MDUPDTYPE_NONE;
            md.m_side = MD_SIDE_NONE;
            md.m_endPkt = 1;

            // Enqueue to MWMR
            g_md_queue->enqueue(md);
            g_md_count++;

            if (g_md_count % 100 == 0) {
                std::cout << "\r[Simulator] " << inst.symbol
                          << " bid=" << md.m_bidUpdates[0].price
                          << " ask=" << md.m_askUpdates[0].price
                          << " total=" << g_md_count.load() << "    " << std::flush;
            }
        }

        std::this_thread::sleep_for(sleep_us);
    }

    std::cout << "\n[Simulator] Stopped. Total ticks: " << g_md_count.load() << std::endl;
}

// ============================================================
// CTP mode — receives real CTP market data
// ============================================================
#if defined(ENABLE_CTP_MD)

class CTPMDFeeder : public CThostFtdcMdSpi {
public:
    CTPMDFeeder(const std::string& config_file, const std::vector<std::string>& symbols)
        : m_symbols(symbols)
    {
        // Load CTP config
        hft::gateway::CTPMDConfig config;
        if (!config.LoadFromYaml(config_file)) {
            throw std::runtime_error("Failed to load CTP MD config: " + config_file);
        }
        m_broker_id = config.broker_id;
        m_user_id = config.user_id;
        m_password = config.password;
        m_front_addr = config.front_addr;

        if (m_symbols.empty()) {
            m_symbols = config.instruments;
        }

        m_api = CThostFtdcMdApi::CreateFtdcMdApi("./ctp_flow/");
        m_api->RegisterSpi(this);
    }

    ~CTPMDFeeder() {
        if (m_api) {
            m_api->Release();
            m_api = nullptr;
        }
    }

    void Start() {
        std::cout << "[CTP-MD] Connecting to " << m_front_addr << std::endl;
        m_api->RegisterFront(const_cast<char*>(m_front_addr.c_str()));
        m_api->Init();
    }

    void WaitForExit() {
        while (g_running.load()) {
            std::this_thread::sleep_for(std::chrono::seconds(1));
        }
    }

    // CTP callbacks
    void OnFrontConnected() override {
        std::cout << "[CTP-MD] Connected, logging in..." << std::endl;
        CThostFtdcReqUserLoginField req = {};
        strncpy(req.BrokerID, m_broker_id.c_str(), sizeof(req.BrokerID) - 1);
        strncpy(req.UserID, m_user_id.c_str(), sizeof(req.UserID) - 1);
        strncpy(req.Password, m_password.c_str(), sizeof(req.Password) - 1);
        m_api->ReqUserLogin(&req, ++m_request_id);
    }

    void OnFrontDisconnected(int nReason) override {
        std::cerr << "[CTP-MD] Disconnected, reason=" << nReason << std::endl;
    }

    void OnRspUserLogin(CThostFtdcRspUserLoginField* pRspUserLogin,
                        CThostFtdcRspInfoField* pRspInfo,
                        int nRequestID, bool bIsLast) override {
        if (pRspInfo && pRspInfo->ErrorID != 0) {
            std::cerr << "[CTP-MD] Login failed: " << pRspInfo->ErrorMsg << std::endl;
            return;
        }
        std::cout << "[CTP-MD] Login successful" << std::endl;
        if (pRspUserLogin) {
            std::cout << "[CTP-MD] Trading day: " << pRspUserLogin->TradingDay << std::endl;
        }

        // Subscribe
        std::vector<char*> ids;
        for (auto& s : m_symbols) {
            ids.push_back(const_cast<char*>(s.c_str()));
        }
        m_api->SubscribeMarketData(ids.data(), static_cast<int>(ids.size()));
        std::cout << "[CTP-MD] Subscribed to " << ids.size() << " instruments" << std::endl;
    }

    void OnRtnDepthMarketData(CThostFtdcDepthMarketDataField* pData) override {
        if (!pData || !g_md_queue) return;

        MarketUpdateNew md;
        std::memset(&md, 0, sizeof(md));

        // Header
        auto now_ns = std::chrono::duration_cast<std::chrono::nanoseconds>(
            std::chrono::system_clock::now().time_since_epoch()).count();
        md.m_exchTS = now_ns;
        md.m_timestamp = now_ns;
        md.m_seqnum = ++m_seq;
        std::strncpy(md.m_symbol, pData->InstrumentID, sizeof(md.m_symbol) - 1);

        // Exchange: try ExchangeID field, fallback to guess
        std::string exchange_id(pData->ExchangeID);
        if (!exchange_id.empty()) {
            md.m_exchangeName = ExchangeNameFromString(exchange_id);
        } else {
            md.m_exchangeName = GuessExchangeFromSymbol(pData->InstrumentID);
        }

        // Convert 5-level depth (CTP provides BidPrice1-5, AskPrice1-5)
        auto set_level = [](double price, int qty, bookElement_t& elem) {
            if (price > 0 && price < 1e10) {
                elem.price = price;
                elem.quantity = qty;
                elem.orderCount = 1;
            }
        };

        int valid_bids = 0;
        set_level(pData->BidPrice1, pData->BidVolume1, md.m_bidUpdates[0]); if (pData->BidPrice1 > 0 && pData->BidPrice1 < 1e10) valid_bids = 1;
        set_level(pData->BidPrice2, pData->BidVolume2, md.m_bidUpdates[1]); if (pData->BidPrice2 > 0 && pData->BidPrice2 < 1e10) valid_bids = 2;
        set_level(pData->BidPrice3, pData->BidVolume3, md.m_bidUpdates[2]); if (pData->BidPrice3 > 0 && pData->BidPrice3 < 1e10) valid_bids = 3;
        set_level(pData->BidPrice4, pData->BidVolume4, md.m_bidUpdates[3]); if (pData->BidPrice4 > 0 && pData->BidPrice4 < 1e10) valid_bids = 4;
        set_level(pData->BidPrice5, pData->BidVolume5, md.m_bidUpdates[4]); if (pData->BidPrice5 > 0 && pData->BidPrice5 < 1e10) valid_bids = 5;

        int valid_asks = 0;
        set_level(pData->AskPrice1, pData->AskVolume1, md.m_askUpdates[0]); if (pData->AskPrice1 > 0 && pData->AskPrice1 < 1e10) valid_asks = 1;
        set_level(pData->AskPrice2, pData->AskVolume2, md.m_askUpdates[1]); if (pData->AskPrice2 > 0 && pData->AskPrice2 < 1e10) valid_asks = 2;
        set_level(pData->AskPrice3, pData->AskVolume3, md.m_askUpdates[2]); if (pData->AskPrice3 > 0 && pData->AskPrice3 < 1e10) valid_asks = 3;
        set_level(pData->AskPrice4, pData->AskVolume4, md.m_askUpdates[3]); if (pData->AskPrice4 > 0 && pData->AskPrice4 < 1e10) valid_asks = 4;
        set_level(pData->AskPrice5, pData->AskVolume5, md.m_askUpdates[4]); if (pData->AskPrice5 > 0 && pData->AskPrice5 < 1e10) valid_asks = 5;

        md.m_validBids = static_cast<int8_t>(valid_bids);
        md.m_validAsks = static_cast<int8_t>(valid_asks);

        // Last traded price
        if (pData->LastPrice > 0 && pData->LastPrice < 1e10) {
            md.m_lastTradedPrice = pData->LastPrice;
        }
        md.m_lastTradedQuantity = pData->Volume;  // CTP Volume is cumulative
        md.m_totalTradedQuantity = pData->Volume;
        md.m_totalTradedValue = pData->Turnover;

        md.m_feedType = FEED_SNAPSHOT;
        md.m_updateType = MDUPDTYPE_NONE;
        md.m_side = MD_SIDE_NONE;
        md.m_endPkt = 1;

        // Enqueue
        g_md_queue->enqueue(md);
        g_md_count++;

        if (g_md_count % 100 == 0) {
            std::cout << "\r[CTP-MD] " << pData->InstrumentID
                      << " bid=" << pData->BidPrice1
                      << " ask=" << pData->AskPrice1
                      << " total=" << g_md_count.load() << "    " << std::flush;
        }
    }

private:
    CThostFtdcMdApi* m_api = nullptr;
    std::vector<std::string> m_symbols;
    std::string m_broker_id;
    std::string m_user_id;
    std::string m_password;
    std::string m_front_addr;
    int m_request_id = 0;
    uint64_t m_seq = 0;
};

#endif // ENABLE_CTP_MD

// ============================================================
// main
// ============================================================
int main(int argc, char** argv) {
    std::cout << R"(
╔════════════════════════════════════════════════════════╗
║   MD SHM Feeder — MarketUpdateNew to SysV MWMR        ║
║   Feeds Go trader directly via shared memory           ║
╚════════════════════════════════════════════════════════╝
)" << std::endl;

    if (argc < 2) {
        std::cerr << "Usage: " << argv[0] << " <mode:config> [options]" << std::endl;
        std::cerr << "\nModes:" << std::endl;
        std::cerr << "  simulator:sym1,sym2  Generate simulated market data" << std::endl;
        std::cerr << "  ctp:config.yaml      Receive CTP market data" << std::endl;
        std::cerr << "\nOptions:" << std::endl;
        std::cerr << "  --rate N             Ticks per second per symbol (simulator, default 2)" << std::endl;
        std::cerr << "  --queue-size N       MWMR queue size (default 65536, use 2048 on macOS)" << std::endl;
        std::cerr << "\nExamples:" << std::endl;
        std::cerr << "  " << argv[0] << " simulator:ag2506,ag2512" << std::endl;
        std::cerr << "  " << argv[0] << " simulator:ag2506,ag2512 --rate 5 --queue-size 2048" << std::endl;
        std::cerr << "  " << argv[0] << " ctp:config/ctp/ctp_md.secret.yaml" << std::endl;
        return 1;
    }

    signal(SIGINT, SignalHandler);
    signal(SIGTERM, SignalHandler);

    // Parse mode:config
    std::string mode_config(argv[1]);
    size_t colon = mode_config.find(':');
    if (colon == std::string::npos) {
        std::cerr << "[MDFeeder] Invalid format: " << mode_config << std::endl;
        std::cerr << "[MDFeeder] Expected: <mode>:<config>" << std::endl;
        return 1;
    }
    std::string mode = mode_config.substr(0, colon);
    std::string config = mode_config.substr(colon + 1);

    // Parse optional flags
    int rate_hz = 2;
    int queue_size_override = 0;
    for (int i = 2; i < argc; i++) {
        std::string arg(argv[i]);
        if (arg == "--rate" && i + 1 < argc) {
            rate_hz = std::atoi(argv[++i]);
            if (rate_hz <= 0) rate_hz = 2;
        } else if (arg == "--queue-size" && i + 1 < argc) {
            queue_size_override = std::atoi(argv[++i]);
        }
    }

    // Parse symbols (comma-separated)
    auto parseSymbols = [](const std::string& s) -> std::vector<std::string> {
        std::vector<std::string> result;
        std::istringstream ss(s);
        std::string token;
        while (std::getline(ss, token, ',')) {
            if (!token.empty()) result.push_back(token);
        }
        return result;
    };

    // 1. Create SysV MWMR MD queue
    MDFeederConfig cfg;
    if (queue_size_override > 0) {
        cfg.md_queue_size = queue_size_override;
    }
    std::cout << "[MDFeeder] Creating SysV MWMR MD queue..." << std::endl;
    std::cout << "[MDFeeder]   Key: 0x" << std::hex << cfg.md_shm_key << std::dec << std::endl;
    std::cout << "[MDFeeder]   Size: " << cfg.md_queue_size << " elements" << std::endl;
    std::cout << "[MDFeeder]   Elem size: " << sizeof(MarketUpdateNew) << " + 8 (seqNo) = "
              << sizeof(QueueElem<MarketUpdateNew>) << " bytes" << std::endl;

    try {
        g_md_queue = MDQueue::Create(cfg.md_shm_key, cfg.md_queue_size);
    } catch (const std::exception& e) {
        std::cerr << "[MDFeeder] Failed to create MD queue: " << e.what() << std::endl;
        return 1;
    }

    std::cout << "[MDFeeder] MD queue ready" << std::endl;

    // 2. Run in selected mode
    if (mode == "simulator") {
        auto symbols = parseSymbols(config);
        if (symbols.empty()) {
            std::cerr << "[MDFeeder] No symbols specified" << std::endl;
            return 1;
        }
        RunSimulator(symbols, rate_hz);

    } else if (mode == "ctp") {
#if defined(ENABLE_CTP_MD)
        auto symbols = parseSymbols(config);
        // If config looks like a YAML file, treat it as config; symbols come from config
        std::string config_file;
        if (config.find(".yaml") != std::string::npos || config.find(".yml") != std::string::npos) {
            config_file = config;
            symbols.clear();  // symbols from config
        }

        CTPMDFeeder feeder(config_file, symbols);
        feeder.Start();
        feeder.WaitForExit();
#else
        std::cerr << "[MDFeeder] CTP mode not available (compile with ENABLE_CTP_MD)" << std::endl;
        return 1;
#endif
    } else {
        std::cerr << "[MDFeeder] Unknown mode: " << mode << std::endl;
        return 1;
    }

    // Cleanup
    if (g_md_queue) g_md_queue->close();
    std::cout << "[MDFeeder] Stopped. Total ticks: " << g_md_count.load() << std::endl;
    return 0;
}
