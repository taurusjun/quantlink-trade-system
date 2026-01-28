#include "ctp_td_plugin.h"
#include <iostream>
#include <thread>
#include <chrono>
#include <cstring>
#include <ctime>
#include <iomanip>
#include <sstream>

namespace hft {
namespace plugin {
namespace ctp {

// ==================== 构造和析构 ====================

CTPTDPlugin::CTPTDPlugin() {
    std::cout << "[CTPTDPlugin] Constructor called" << std::endl;
}

CTPTDPlugin::~CTPTDPlugin() {
    Logout();

    if (m_api) {
        m_api->Release();
        m_api = nullptr;
    }
}

// ==================== ITDPlugin接口实现 - 生命周期管理 ====================

bool CTPTDPlugin::Initialize(const std::string& config_file) {
    std::cout << "[CTPTDPlugin] Initializing with config: " << config_file << std::endl;

    try {
        // 加载配置
        if (!m_config.LoadFromYaml(config_file)) {
            std::cerr << "[CTPTDPlugin] ❌ Failed to load config file: " << config_file << std::endl;
            return false;
        }

        // 验证配置
        std::string error;
        if (!m_config.Validate(&error)) {
            std::cerr << "[CTPTDPlugin] ❌ Invalid config: " << error << std::endl;
            return false;
        }

        // 打印配置
        m_config.Print();

        // 创建CTP API实例（流文件保存在./ctp_flow/目录）
        m_api = CThostFtdcTraderApi::CreateFtdcTraderApi("./ctp_flow/");
        if (!m_api) {
            std::cerr << "[CTPTDPlugin] ❌ Failed to create CTP Trader API" << std::endl;
            return false;
        }

        // 注册回调
        m_api->RegisterSpi(this);

        // 注册前置地址
        m_api->RegisterFront(const_cast<char*>(m_config.front_addr.c_str()));

        // 订阅私有流和公有流
        m_api->SubscribePrivateTopic(THOST_TERT_QUICK);
        m_api->SubscribePublicTopic(THOST_TERT_QUICK);

        std::cout << "[CTPTDPlugin] ✅ Initialized successfully" << std::endl;
        return true;

    } catch (const std::exception& e) {
        std::cerr << "[CTPTDPlugin] ❌ Exception during initialization: "
                  << e.what() << std::endl;
        return false;
    }
}

bool CTPTDPlugin::Login() {
    if (!m_api) {
        std::cerr << "[CTPTDPlugin] ❌ Cannot login: not initialized" << std::endl;
        return false;
    }

    if (m_logged_in.load()) {
        std::cout << "[CTPTDPlugin] Already logged in" << std::endl;
        return true;
    }

    std::cout << "[CTPTDPlugin] Starting login process..." << std::endl;
    std::cout << "[CTPTDPlugin] Connecting to " << m_config.front_addr << std::endl;

    // 初始化（会触发OnFrontConnected回调）
    m_api->Init();

    // 等待登录完成（超时30秒）
    int wait_count = 0;
    while (!m_logged_in.load() && wait_count < 300) {
        std::this_thread::sleep_for(std::chrono::milliseconds(100));
        wait_count++;
    }

    if (m_logged_in.load()) {
        std::cout << "[CTPTDPlugin] ✅ Login successful" << std::endl;
        return true;
    } else {
        std::cerr << "[CTPTDPlugin] ❌ Login timeout after 30 seconds" << std::endl;
        return false;
    }
}

void CTPTDPlugin::Logout() {
    if (!m_logged_in.load()) {
        return;
    }

    std::cout << "[CTPTDPlugin] Logging out..." << std::endl;

    if (m_api) {
        CThostFtdcUserLogoutField req = {};
        strncpy(req.BrokerID, m_config.broker_id.c_str(), sizeof(req.BrokerID) - 1);
        strncpy(req.UserID, m_config.user_id.c_str(), sizeof(req.UserID) - 1);

        m_api->ReqUserLogout(&req, ++m_request_id);
    }

    m_logged_in.store(false);
    m_authenticated.store(false);
    m_settlement_confirmed.store(false);
    m_connected.store(false);

    std::cout << "[CTPTDPlugin] Logged out" << std::endl;
}

// ==================== CTP API回调 - 连接相关 ====================

void CTPTDPlugin::OnFrontConnected() {
    std::cout << "[CTPTDPlugin] ✅ Connected to front server" << std::endl;
    m_connected.store(true);
    m_reconnect_count = 0;  // 重置重连计数

    // 连接成功后进行认证（如果配置了app_id）
    if (!m_config.app_id.empty()) {
        Authenticate();
    } else {
        // 不需要认证，直接登录
        DoLogin();
    }
}

void CTPTDPlugin::OnFrontDisconnected(int nReason) {
    std::cerr << "[CTPTDPlugin] ❌ Disconnected from front server, reason: " << nReason << std::endl;
    m_connected.store(false);
    m_authenticated.store(false);
    m_logged_in.store(false);
    m_settlement_confirmed.store(false);

    // 常见原因说明
    switch (nReason) {
        case 0x1001:
            std::cerr << "  Reason: Network read failure" << std::endl;
            break;
        case 0x1002:
            std::cerr << "  Reason: Network write failure" << std::endl;
            break;
        case 0x2001:
            std::cerr << "  Reason: Heartbeat timeout" << std::endl;
            break;
        case 0x2002:
            std::cerr << "  Reason: Server sent disconnect notification" << std::endl;
            break;
        case 0x2003:
            std::cerr << "  Reason: Repeat login" << std::endl;
            break;
        default:
            std::cerr << "  Reason: Unknown (" << std::hex << nReason << std::dec << ")" << std::endl;
    }

    // 触发重连
    Reconnect();
}

// ==================== 内部方法 - 认证和登录 ====================

void CTPTDPlugin::Authenticate() {
    std::cout << "[CTPTDPlugin] Sending authentication request..." << std::endl;

    CThostFtdcReqAuthenticateField req = {};
    strncpy(req.BrokerID, m_config.broker_id.c_str(), sizeof(req.BrokerID) - 1);
    strncpy(req.UserID, m_config.user_id.c_str(), sizeof(req.UserID) - 1);
    strncpy(req.AppID, m_config.app_id.c_str(), sizeof(req.AppID) - 1);
    strncpy(req.AuthCode, m_config.auth_code.c_str(), sizeof(req.AuthCode) - 1);

    if (!m_config.product_info.empty()) {
        strncpy(req.UserProductInfo, m_config.product_info.c_str(), sizeof(req.UserProductInfo) - 1);
    }

    int ret = m_api->ReqAuthenticate(&req, ++m_request_id);
    if (ret != 0) {
        std::cerr << "[CTPTDPlugin] ❌ Failed to send authentication request, error: " << ret << std::endl;
    }
}

void CTPTDPlugin::OnRspAuthenticate(CThostFtdcRspAuthenticateField* pRspAuthenticateField,
                                    CThostFtdcRspInfoField* pRspInfo,
                                    int nRequestID, bool bIsLast) {
    if (IsErrorResponse(pRspInfo)) {
        std::cerr << "[CTPTDPlugin] ❌ Authentication failed: " << pRspInfo->ErrorMsg
                  << " (ErrorID: " << pRspInfo->ErrorID << ")" << std::endl;
        return;
    }

    std::cout << "[CTPTDPlugin] ✅ Authentication successful" << std::endl;
    m_authenticated.store(true);

    // 认证成功后登录
    DoLogin();
}

void CTPTDPlugin::DoLogin() {
    std::cout << "[CTPTDPlugin] Sending login request..." << std::endl;

    CThostFtdcReqUserLoginField req = {};
    strncpy(req.BrokerID, m_config.broker_id.c_str(), sizeof(req.BrokerID) - 1);
    strncpy(req.UserID, m_config.user_id.c_str(), sizeof(req.UserID) - 1);
    strncpy(req.Password, m_config.password.c_str(), sizeof(req.Password) - 1);

    // 系统信息（新版CTP API要求）
    char systemInfo[256] = {};
    int ret = m_api->ReqUserLogin(&req, ++m_request_id, sizeof(systemInfo), systemInfo);
    if (ret != 0) {
        std::cerr << "[CTPTDPlugin] ❌ Failed to send login request, error: " << ret << std::endl;
    }
}

void CTPTDPlugin::OnRspUserLogin(CThostFtdcRspUserLoginField* pRspUserLogin,
                                 CThostFtdcRspInfoField* pRspInfo,
                                 int nRequestID, bool bIsLast) {
    if (IsErrorResponse(pRspInfo)) {
        std::cerr << "[CTPTDPlugin] ❌ Login failed: " << pRspInfo->ErrorMsg
                  << " (ErrorID: " << pRspInfo->ErrorID << ")" << std::endl;
        return;
    }

    std::cout << "[CTPTDPlugin] ✅ Login successful" << std::endl;
    if (pRspUserLogin) {
        std::cout << "  Trading Day: " << pRspUserLogin->TradingDay << std::endl;
        std::cout << "  Login Time: " << pRspUserLogin->LoginTime << std::endl;
        std::cout << "  System Name: " << pRspUserLogin->SystemName << std::endl;

        // 保存会话信息
        m_front_id = pRspUserLogin->FrontID;
        m_session_id = pRspUserLogin->SessionID;
        m_order_ref.store(atoi(pRspUserLogin->MaxOrderRef));

        std::cout << "  Front ID: " << m_front_id << std::endl;
        std::cout << "  Session ID: " << m_session_id << std::endl;
        std::cout << "  Max Order Ref: " << m_order_ref.load() << std::endl;
    }

    m_logged_in.store(true);

    // 登录成功后确认结算单
    ConfirmSettlement();
}

void CTPTDPlugin::OnRspUserLogout(CThostFtdcUserLogoutField* pUserLogout,
                                  CThostFtdcRspInfoField* pRspInfo,
                                  int nRequestID, bool bIsLast) {
    if (IsErrorResponse(pRspInfo)) {
        std::cerr << "[CTPTDPlugin] ❌ Logout failed: " << pRspInfo->ErrorMsg << std::endl;
        return;
    }

    std::cout << "[CTPTDPlugin] ✅ Logout successful" << std::endl;
    m_logged_in.store(false);
    m_authenticated.store(false);
    m_settlement_confirmed.store(false);
}

// ==================== 结算单确认 ====================

void CTPTDPlugin::ConfirmSettlement() {
    std::cout << "[CTPTDPlugin] Confirming settlement info..." << std::endl;

    CThostFtdcSettlementInfoConfirmField req = {};
    strncpy(req.BrokerID, m_config.broker_id.c_str(), sizeof(req.BrokerID) - 1);
    strncpy(req.InvestorID, m_config.investor_id.c_str(), sizeof(req.InvestorID) - 1);

    int ret = m_api->ReqSettlementInfoConfirm(&req, ++m_request_id);
    if (ret != 0) {
        std::cerr << "[CTPTDPlugin] ❌ Failed to confirm settlement, error: " << ret << std::endl;
    }
}

void CTPTDPlugin::OnRspSettlementInfoConfirm(CThostFtdcSettlementInfoConfirmField* pSettlementInfoConfirm,
                                             CThostFtdcRspInfoField* pRspInfo,
                                             int nRequestID, bool bIsLast) {
    if (IsErrorResponse(pRspInfo)) {
        std::cerr << "[CTPTDPlugin] ❌ Settlement confirmation failed: " << pRspInfo->ErrorMsg << std::endl;
        return;
    }

    std::cout << "[CTPTDPlugin] ✅ Settlement confirmed" << std::endl;
    if (pSettlementInfoConfirm) {
        std::cout << "  Confirm Date: " << pSettlementInfoConfirm->ConfirmDate << std::endl;
        std::cout << "  Confirm Time: " << pSettlementInfoConfirm->ConfirmTime << std::endl;
    }

    m_settlement_confirmed.store(true);
}

// ==================== 辅助方法 ====================

void CTPTDPlugin::Reconnect() {
    m_reconnect_count++;

    // 检查重连次数限制
    if (m_config.max_reconnect_attempts > 0 &&
        m_reconnect_count > m_config.max_reconnect_attempts) {
        std::cerr << "[CTPTDPlugin] ❌ Max reconnect attempts (" << m_config.max_reconnect_attempts
                  << ") reached, giving up" << std::endl;
        return;
    }

    // 限制重连频率
    auto now = std::chrono::steady_clock::now();
    auto elapsed = std::chrono::duration_cast<std::chrono::seconds>(
        now - m_last_reconnect_time
    ).count();

    if (elapsed < m_config.reconnect_interval_sec) {
        int wait_time = m_config.reconnect_interval_sec - elapsed;
        std::cout << "[CTPTDPlugin] Waiting " << wait_time << "s before reconnect..." << std::endl;
        std::this_thread::sleep_for(std::chrono::seconds(wait_time));
    }

    m_last_reconnect_time = now;

    std::cout << "[CTPTDPlugin] Reconnecting (attempt " << m_reconnect_count << ")..." << std::endl;

    // CTP API的重连需要重新初始化
    if (m_api) {
        m_api->Release();
        m_api = nullptr;
    }

    m_api = CThostFtdcTraderApi::CreateFtdcTraderApi("./ctp_flow/");
    m_api->RegisterSpi(this);
    m_api->RegisterFront(const_cast<char*>(m_config.front_addr.c_str()));
    m_api->SubscribePrivateTopic(THOST_TERT_QUICK);
    m_api->SubscribePublicTopic(THOST_TERT_QUICK);
    m_api->Init();
}

bool CTPTDPlugin::IsErrorResponse(CThostFtdcRspInfoField* pRspInfo) {
    return pRspInfo && pRspInfo->ErrorID != 0;
}

void CTPTDPlugin::OnRspError(CThostFtdcRspInfoField* pRspInfo,
                             int nRequestID, bool bIsLast) {
    if (pRspInfo && pRspInfo->ErrorID != 0) {
        std::cerr << "[CTPTDPlugin] Error Response: " << pRspInfo->ErrorMsg
                  << " (ErrorID: " << pRspInfo->ErrorID << ")" << std::endl;

        // 触发错误回调
        std::lock_guard<std::mutex> lock(m_callback_mutex);
        if (m_error_callback) {
            m_error_callback(pRspInfo->ErrorID, pRspInfo->ErrorMsg);
        }
    }
}

std::string CTPTDPlugin::GenerateOrderRef() {
    int ref = ++m_order_ref;
    std::ostringstream oss;
    oss << std::setw(12) << std::setfill('0') << ref;
    return oss.str();
}

// ==================== 回调注册 ====================

void CTPTDPlugin::RegisterOrderCallback(OrderCallback callback) {
    std::lock_guard<std::mutex> lock(m_callback_mutex);
    m_order_callback = callback;
}

void CTPTDPlugin::RegisterTradeCallback(TradeCallback callback) {
    std::lock_guard<std::mutex> lock(m_callback_mutex);
    m_trade_callback = callback;
}

void CTPTDPlugin::RegisterErrorCallback(ErrorCallback callback) {
    std::lock_guard<std::mutex> lock(m_callback_mutex);
    m_error_callback = callback;
}

double CTPTDPlugin::GetAvailableFund() const {
    return m_available_fund.load();
}


// ==================== 下单和撤单功能 ====================

std::string CTPTDPlugin::SendOrder(const OrderRequest& request) {
    if (!m_settlement_confirmed.load()) {
        std::cerr << "[CTPTDPlugin] ❌ Cannot send order: settlement not confirmed" << std::endl;
        return "";
    }

    // 生成订单引用
    std::string order_ref = GenerateOrderRef();

    std::cout << "[CTPTDPlugin] Sending order: " << request.symbol
              << " " << (request.direction == OrderDirection::BUY ? "BUY" : "SELL")
              << " " << request.volume << "@" << request.price << std::endl;

    // 构建CTP报单请求
    CThostFtdcInputOrderField req = {};

    // 经纪商和投资者
    strncpy(req.BrokerID, m_config.broker_id.c_str(), sizeof(req.BrokerID) - 1);
    strncpy(req.InvestorID, m_config.investor_id.c_str(), sizeof(req.InvestorID) - 1);

    // 合约
    strncpy(req.InstrumentID, request.symbol, sizeof(req.InstrumentID) - 1);
    strncpy(req.ExchangeID, request.exchange, sizeof(req.ExchangeID) - 1);

    // 报单引用
    strncpy(req.OrderRef, order_ref.c_str(), sizeof(req.OrderRef) - 1);

    // 买卖方向
    req.Direction = (request.direction == OrderDirection::BUY) ? THOST_FTDC_D_Buy : THOST_FTDC_D_Sell;

    // 组合开平标志
    switch (request.offset) {
        case OffsetFlag::OPEN:
            req.CombOffsetFlag[0] = THOST_FTDC_OF_Open;
            break;
        case OffsetFlag::CLOSE:
            req.CombOffsetFlag[0] = THOST_FTDC_OF_Close;
            break;
        case OffsetFlag::CLOSE_TODAY:
            req.CombOffsetFlag[0] = THOST_FTDC_OF_CloseToday;
            break;
        case OffsetFlag::CLOSE_YESTERDAY:
            req.CombOffsetFlag[0] = THOST_FTDC_OF_CloseYesterday;
            break;
    }

    // 组合投机套保标志
    req.CombHedgeFlag[0] = THOST_FTDC_HF_Speculation;  // 投机

    // 价格
    req.LimitPrice = request.price;

    // 数量
    req.VolumeTotalOriginal = request.volume;

    // 有效期类型：当日有效
    req.TimeCondition = THOST_FTDC_TC_GFD;

    // 成交量类型：任何数量
    req.VolumeCondition = THOST_FTDC_VC_AV;

    // 最小成交量
    req.MinVolume = 1;

    // 触发条件：立即
    req.ContingentCondition = THOST_FTDC_CC_Immediately;

    // 强平原因：非强平
    req.ForceCloseReason = THOST_FTDC_FCC_NotForceClose;

    // 自动挂起标志：否
    req.IsAutoSuspend = 0;

    // 用户强平标志：否
    req.UserForceClose = 0;

    // 价格类型
    switch (request.price_type) {
        case PriceType::LIMIT:
            req.OrderPriceType = THOST_FTDC_OPT_LimitPrice;
            break;
        case PriceType::MARKET:
            req.OrderPriceType = THOST_FTDC_OPT_AnyPrice;
            req.LimitPrice = 0.0;  // 市价单价格填0
            break;
        case PriceType::BEST:
            req.OrderPriceType = THOST_FTDC_OPT_BestPrice;
            break;
    }

    // 发送报单
    int ret = m_api->ReqOrderInsert(&req, ++m_request_id);
    if (ret != 0) {
        std::cerr << "[CTPTDPlugin] ❌ Failed to send order, error: " << ret << std::endl;
        return "";
    }

    // 构造订单ID（FrontID-SessionID-OrderRef）
    std::ostringstream oss;
    oss << m_front_id << "-" << m_session_id << "-" << order_ref;
    std::string order_id = oss.str();

    // 保存订单到本地缓存（状态为提交中）
    OrderInfo order_info;
    strncpy(order_info.order_id, order_id.c_str(), sizeof(order_info.order_id) - 1);
    if (request.client_order_id[0] != '\0') {
        strncpy(order_info.client_order_id, request.client_order_id, sizeof(order_info.client_order_id) - 1);
    }
    strncpy(order_info.symbol, request.symbol, sizeof(order_info.symbol) - 1);
    strncpy(order_info.exchange, request.exchange, sizeof(order_info.exchange) - 1);
    order_info.direction = request.direction;
    order_info.offset = request.offset;
    order_info.price_type = request.price_type;
    order_info.price = request.price;
    order_info.volume = request.volume;
    order_info.traded_volume = 0;
    order_info.status = OrderStatus::SUBMITTING;
    order_info.insert_time = std::chrono::duration_cast<std::chrono::nanoseconds>(
        std::chrono::system_clock::now().time_since_epoch()
    ).count();

    SaveOrder(order_id, order_info);

    std::cout << "[CTPTDPlugin] Order submitted with ID: " << order_id << std::endl;

    m_order_count.fetch_add(1);

    return order_id;
}

bool CTPTDPlugin::CancelOrder(const std::string& order_id) {
    if (!m_settlement_confirmed.load()) {
        std::cerr << "[CTPTDPlugin] ❌ Cannot cancel order: settlement not confirmed" << std::endl;
        return false;
    }

    // 从缓存中获取订单信息
    OrderInfo order_info;
    if (!GetOrderFromCache(order_id, order_info)) {
        std::cerr << "[CTPTDPlugin] ❌ Order not found: " << order_id << std::endl;
        return false;
    }

    std::cout << "[CTPTDPlugin] Canceling order: " << order_id << std::endl;

    // 解析order_id (FrontID-SessionID-OrderRef)
    std::istringstream iss(order_id);
    std::string token;
    std::vector<std::string> parts;
    while (std::getline(iss, token, '-')) {
        parts.push_back(token);
    }

    if (parts.size() != 3) {
        std::cerr << "[CTPTDPlugin] ❌ Invalid order ID format: " << order_id << std::endl;
        return false;
    }

    // 构建撤单请求
    CThostFtdcInputOrderActionField req = {};
    strncpy(req.BrokerID, m_config.broker_id.c_str(), sizeof(req.BrokerID) - 1);
    strncpy(req.InvestorID, m_config.investor_id.c_str(), sizeof(req.InvestorID) - 1);
    strncpy(req.OrderRef, parts[2].c_str(), sizeof(req.OrderRef) - 1);
    req.FrontID = std::stoi(parts[0]);
    req.SessionID = std::stoi(parts[1]);
    req.ActionFlag = THOST_FTDC_AF_Delete;

    strncpy(req.InstrumentID, order_info.symbol, sizeof(req.InstrumentID) - 1);

    int ret = m_api->ReqOrderAction(&req, ++m_request_id);
    if (ret != 0) {
        std::cerr << "[CTPTDPlugin] ❌ Failed to cancel order, error: " << ret << std::endl;
        return false;
    }

    std::cout << "[CTPTDPlugin] Cancel request sent for order: " << order_id << std::endl;
    return true;
}

// ==================== 报单回调 ====================

void CTPTDPlugin::OnRspOrderInsert(CThostFtdcInputOrderField* pInputOrder,
                                   CThostFtdcRspInfoField* pRspInfo,
                                   int nRequestID, bool bIsLast) {
    if (IsErrorResponse(pRspInfo)) {
        std::cerr << "[CTPTDPlugin] ❌ Order insert failed: " << pRspInfo->ErrorMsg
                  << " (ErrorID: " << pRspInfo->ErrorID << ")" << std::endl;

        if (pInputOrder) {
            // 构造订单ID
            std::ostringstream oss;
            oss << m_front_id << "-" << m_session_id << "-" << pInputOrder->OrderRef;
            std::string order_id = oss.str();

            // 更新订单状态为拒绝
            OrderInfo order_info;
            if (GetOrderFromCache(order_id, order_info)) {
                order_info.status = OrderStatus::REJECTED;
                strncpy(order_info.status_msg, pRspInfo->ErrorMsg, sizeof(order_info.status_msg) - 1);
                SaveOrder(order_id, order_info);

                // 触发订单回调
                std::lock_guard<std::mutex> lock(m_callback_mutex);
                if (m_order_callback) {
                    m_order_callback(order_info);
                }
            }
        }
    }
}

void CTPTDPlugin::OnRspOrderAction(CThostFtdcInputOrderActionField* pInputOrderAction,
                                   CThostFtdcRspInfoField* pRspInfo,
                                   int nRequestID, bool bIsLast) {
    if (IsErrorResponse(pRspInfo)) {
        std::cerr << "[CTPTDPlugin] ❌ Order cancel failed: " << pRspInfo->ErrorMsg
                  << " (ErrorID: " << pRspInfo->ErrorID << ")" << std::endl;
    }
}

void CTPTDPlugin::OnRtnOrder(CThostFtdcOrderField* pOrder) {
    if (!pOrder) {
        return;
    }

    // 转换订单信息
    OrderInfo order_info;
    ConvertOrder(pOrder, order_info);

    // 保存到本地缓存
    SaveOrder(order_info.order_id, order_info);

    std::cout << "[CTPTDPlugin] Order update: " << order_info.order_id
              << " status=" << static_cast<int>(order_info.status)
              << " traded=" << order_info.traded_volume << "/" << order_info.volume << std::endl;

    // 触发订单回调
    std::lock_guard<std::mutex> lock(m_callback_mutex);
    if (m_order_callback) {
        m_order_callback(order_info);
    }
}

void CTPTDPlugin::OnRtnTrade(CThostFtdcTradeField* pTrade) {
    if (!pTrade) {
        return;
    }

    // 转换成交信息
    TradeInfo trade_info;
    ConvertTrade(pTrade, trade_info);

    std::cout << "[CTPTDPlugin] Trade: " << trade_info.trade_id
              << " " << trade_info.symbol
              << " " << (trade_info.direction == OrderDirection::BUY ? "BUY" : "SELL")
              << " " << trade_info.volume << "@" << trade_info.price << std::endl;

    m_trade_count.fetch_add(1);

    // 触发成交回调
    std::lock_guard<std::mutex> lock(m_callback_mutex);
    if (m_trade_callback) {
        m_trade_callback(trade_info);
    }
}


// ==================== 查询功能 ====================

bool CTPTDPlugin::QueryAccount(AccountInfo& account_info) {
    if (!m_logged_in.load()) {
        std::cerr << "[CTPTDPlugin] ❌ Cannot query: not logged in" << std::endl;
        return false;
    }

    std::lock_guard<std::mutex> lock(m_query_mutex);
    m_query_finished = false;

    CThostFtdcQryTradingAccountField req = {};
    strncpy(req.BrokerID, m_config.broker_id.c_str(), sizeof(req.BrokerID) - 1);
    strncpy(req.InvestorID, m_config.investor_id.c_str(), sizeof(req.InvestorID) - 1);

    int ret = m_api->ReqQryTradingAccount(&req, ++m_request_id);
    if (ret != 0) {
        std::cerr << "[CTPTDPlugin] ❌ Failed to query account, error: " << ret << std::endl;
        return false;
    }

    // 等待查询完成（超时5秒）
    std::unique_lock<std::mutex> ulock(m_query_mutex);
    m_query_cv.wait_for(ulock, std::chrono::seconds(5), [this] { return m_query_finished; });

    if (!m_query_finished) {
        std::cerr << "[CTPTDPlugin] ❌ Query account timeout" << std::endl;
        return false;
    }

    account_info = m_cached_account;
    return true;
}

void CTPTDPlugin::OnRspQryTradingAccount(CThostFtdcTradingAccountField* pTradingAccount,
                                         CThostFtdcRspInfoField* pRspInfo,
                                         int nRequestID, bool bIsLast) {
    if (IsErrorResponse(pRspInfo)) {
        std::cerr << "[CTPTDPlugin] ❌ Query account failed: " << pRspInfo->ErrorMsg << std::endl;
        std::lock_guard<std::mutex> lock(m_query_mutex);
        m_query_finished = true;
        m_query_cv.notify_one();
        return;
    }

    if (pTradingAccount) {
        ConvertAccount(pTradingAccount, m_cached_account);
        m_available_fund.store(m_cached_account.available);
    }

    if (bIsLast) {
        std::lock_guard<std::mutex> lock(m_query_mutex);
        m_query_finished = true;
        m_query_cv.notify_one();
    }
}

bool CTPTDPlugin::QueryPositions(std::vector<PositionInfo>& positions) {
    if (!m_logged_in.load()) {
        std::cerr << "[CTPTDPlugin] ❌ Cannot query: not logged in" << std::endl;
        return false;
    }

    // 准备查询（需要锁保护）
    {
        std::lock_guard<std::mutex> lock(m_query_mutex);
        m_query_finished = false;
        m_cached_positions.clear();
    }

    // 发送查询请求
    CThostFtdcQryInvestorPositionField req = {};
    strncpy(req.BrokerID, m_config.broker_id.c_str(), sizeof(req.BrokerID) - 1);
    strncpy(req.InvestorID, m_config.investor_id.c_str(), sizeof(req.InvestorID) - 1);

    int ret = m_api->ReqQryInvestorPosition(&req, ++m_request_id);
    if (ret != 0) {
        std::cerr << "[CTPTDPlugin] ❌ Failed to query positions, error: " << ret << std::endl;
        return false;
    }

    // 等待查询完成
    std::unique_lock<std::mutex> ulock(m_query_mutex);
    m_query_cv.wait_for(ulock, std::chrono::seconds(5), [this] { return m_query_finished; });

    if (!m_query_finished) {
        std::cerr << "[CTPTDPlugin] ❌ Query positions timeout" << std::endl;
        return false;
    }

    positions = m_cached_positions;
    return true;
}

void CTPTDPlugin::OnRspQryInvestorPosition(CThostFtdcInvestorPositionField* pInvestorPosition,
                                           CThostFtdcRspInfoField* pRspInfo,
                                           int nRequestID, bool bIsLast) {
    if (IsErrorResponse(pRspInfo)) {
        std::cerr << "[CTPTDPlugin] ❌ Query positions failed: " << pRspInfo->ErrorMsg << std::endl;
        std::lock_guard<std::mutex> lock(m_query_mutex);
        m_query_finished = true;
        m_query_cv.notify_one();
        return;
    }

    if (pInvestorPosition) {
        PositionInfo pos_info;
        ConvertPosition(pInvestorPosition, pos_info);
        m_cached_positions.push_back(pos_info);
    }

    if (bIsLast) {
        std::lock_guard<std::mutex> lock(m_query_mutex);
        m_query_finished = true;
        m_query_cv.notify_one();
    }
}

bool CTPTDPlugin::QueryOrders(std::vector<OrderInfo>& orders) {
    if (!m_logged_in.load()) {
        std::cerr << "[CTPTDPlugin] ❌ Cannot query: not logged in" << std::endl;
        return false;
    }

    std::lock_guard<std::mutex> lock(m_query_mutex);
    m_query_finished = false;
    m_cached_orders.clear();

    CThostFtdcQryOrderField req = {};
    strncpy(req.BrokerID, m_config.broker_id.c_str(), sizeof(req.BrokerID) - 1);
    strncpy(req.InvestorID, m_config.investor_id.c_str(), sizeof(req.InvestorID) - 1);

    int ret = m_api->ReqQryOrder(&req, ++m_request_id);
    if (ret != 0) {
        std::cerr << "[CTPTDPlugin] ❌ Failed to query orders, error: " << ret << std::endl;
        return false;
    }

    // 等待查询完成
    std::unique_lock<std::mutex> ulock(m_query_mutex);
    m_query_cv.wait_for(ulock, std::chrono::seconds(5), [this] { return m_query_finished; });

    if (!m_query_finished) {
        std::cerr << "[CTPTDPlugin] ❌ Query orders timeout" << std::endl;
        return false;
    }

    orders = m_cached_orders;
    return true;
}

void CTPTDPlugin::OnRspQryOrder(CThostFtdcOrderField* pOrder,
                                CThostFtdcRspInfoField* pRspInfo,
                                int nRequestID, bool bIsLast) {
    if (IsErrorResponse(pRspInfo)) {
        std::cerr << "[CTPTDPlugin] ❌ Query orders failed: " << pRspInfo->ErrorMsg << std::endl;
        std::lock_guard<std::mutex> lock(m_query_mutex);
        m_query_finished = true;
        m_query_cv.notify_one();
        return;
    }

    if (pOrder) {
        OrderInfo order_info;
        ConvertOrder(pOrder, order_info);
        m_cached_orders.push_back(order_info);
    }

    if (bIsLast) {
        std::lock_guard<std::mutex> lock(m_query_mutex);
        m_query_finished = true;
        m_query_cv.notify_one();
    }
}

bool CTPTDPlugin::QueryTrades(std::vector<TradeInfo>& trades) {
    if (!m_logged_in.load()) {
        std::cerr << "[CTPTDPlugin] ❌ Cannot query: not logged in" << std::endl;
        return false;
    }

    std::lock_guard<std::mutex> lock(m_query_mutex);
    m_query_finished = false;
    m_cached_trades.clear();

    CThostFtdcQryTradeField req = {};
    strncpy(req.BrokerID, m_config.broker_id.c_str(), sizeof(req.BrokerID) - 1);
    strncpy(req.InvestorID, m_config.investor_id.c_str(), sizeof(req.InvestorID) - 1);

    int ret = m_api->ReqQryTrade(&req, ++m_request_id);
    if (ret != 0) {
        std::cerr << "[CTPTDPlugin] ❌ Failed to query trades, error: " << ret << std::endl;
        return false;
    }

    // 等待查询完成
    std::unique_lock<std::mutex> ulock(m_query_mutex);
    m_query_cv.wait_for(ulock, std::chrono::seconds(5), [this] { return m_query_finished; });

    if (!m_query_finished) {
        std::cerr << "[CTPTDPlugin] ❌ Query trades timeout" << std::endl;
        return false;
    }

    trades = m_cached_trades;
    return true;
}

void CTPTDPlugin::OnRspQryTrade(CThostFtdcTradeField* pTrade,
                                CThostFtdcRspInfoField* pRspInfo,
                                int nRequestID, bool bIsLast) {
    if (IsErrorResponse(pRspInfo)) {
        std::cerr << "[CTPTDPlugin] ❌ Query trades failed: " << pRspInfo->ErrorMsg << std::endl;
        std::lock_guard<std::mutex> lock(m_query_mutex);
        m_query_finished = true;
        m_query_cv.notify_one();
        return;
    }

    if (pTrade) {
        TradeInfo trade_info;
        ConvertTrade(pTrade, trade_info);
        m_cached_trades.push_back(trade_info);
    }

    if (bIsLast) {
        std::lock_guard<std::mutex> lock(m_query_mutex);
        m_query_finished = true;
        m_query_cv.notify_one();
    }
}

bool CTPTDPlugin::GetOrder(const std::string& order_id, OrderInfo& order_info) {
    return GetOrderFromCache(order_id, order_info);
}


// ==================== 数据转换函数 ====================

void CTPTDPlugin::ConvertOrder(CThostFtdcOrderField* ctp_order, OrderInfo& order_info) {
    if (!ctp_order) return;

    // 构建订单ID: FrontID-SessionID-OrderRef
    snprintf(order_info.order_id, sizeof(order_info.order_id),
             "%d-%d-%s", ctp_order->FrontID, ctp_order->SessionID, ctp_order->OrderRef);

    // 客户端订单ID（如果有）
    if (ctp_order->OrderSysID[0] != '\0') {
        strncpy(order_info.client_order_id, ctp_order->OrderSysID,
                sizeof(order_info.client_order_id) - 1);
    }

    // 合约信息
    strncpy(order_info.symbol, ctp_order->InstrumentID, sizeof(order_info.symbol) - 1);
    strncpy(order_info.exchange, ctp_order->ExchangeID, sizeof(order_info.exchange) - 1);

    // 买卖方向
    order_info.direction = (ctp_order->Direction == THOST_FTDC_D_Buy) ?
        OrderDirection::BUY : OrderDirection::SELL;

    // 开平标志
    switch (ctp_order->CombOffsetFlag[0]) {
        case THOST_FTDC_OF_Open:
            order_info.offset = OffsetFlag::OPEN;
            break;
        case THOST_FTDC_OF_Close:
            order_info.offset = OffsetFlag::CLOSE;
            break;
        case THOST_FTDC_OF_CloseToday:
            order_info.offset = OffsetFlag::CLOSE_TODAY;
            break;
        case THOST_FTDC_OF_CloseYesterday:
            order_info.offset = OffsetFlag::CLOSE_YESTERDAY;
            break;
        default:
            order_info.offset = OffsetFlag::OPEN;
    }

    // 价格类型
    switch (ctp_order->OrderPriceType) {
        case THOST_FTDC_OPT_LimitPrice:
            order_info.price_type = PriceType::LIMIT;
            break;
        case THOST_FTDC_OPT_AnyPrice:
            order_info.price_type = PriceType::MARKET;
            break;
        case THOST_FTDC_OPT_BestPrice:
            order_info.price_type = PriceType::BEST;
            break;
        default:
            order_info.price_type = PriceType::LIMIT;
    }

    order_info.price = ctp_order->LimitPrice;
    order_info.volume = ctp_order->VolumeTotalOriginal;
    order_info.traded_volume = ctp_order->VolumeTraded;

    // 订单状态
    switch (ctp_order->OrderStatus) {
        case THOST_FTDC_OST_AllTraded:
            order_info.status = OrderStatus::FILLED;
            break;
        case THOST_FTDC_OST_PartTradedQueueing:
            order_info.status = OrderStatus::PARTIAL_FILLED;
            break;
        case THOST_FTDC_OST_NoTradeQueueing:
            order_info.status = OrderStatus::ACCEPTED;
            break;
        case THOST_FTDC_OST_Canceled:
            order_info.status = OrderStatus::CANCELED;
            break;
        case THOST_FTDC_OST_Unknown:
            order_info.status = OrderStatus::UNKNOWN;
            break;
        default:
            order_info.status = OrderStatus::UNKNOWN;
    }

    // 时间戳（转换为纳秒）
    // CTP 提供的是日期字符串，这里简化处理
    order_info.insert_time = std::chrono::system_clock::now().time_since_epoch().count();
    order_info.update_time = order_info.insert_time;

    // 状态信息
    if (ctp_order->StatusMsg[0] != '\0') {
        strncpy(order_info.status_msg, ctp_order->StatusMsg, sizeof(order_info.status_msg) - 1);
    }
}

void CTPTDPlugin::ConvertTrade(CThostFtdcTradeField* ctp_trade, TradeInfo& trade_info) {
    if (!ctp_trade) return;

    // 成交ID
    strncpy(trade_info.trade_id, ctp_trade->TradeID, sizeof(trade_info.trade_id) - 1);

    // 订单ID（使用交易所订单号，因为TradeField没有FrontID和SessionID）
    if (ctp_trade->OrderSysID[0] != '\0') {
        strncpy(trade_info.order_id, ctp_trade->OrderSysID, sizeof(trade_info.order_id) - 1);
    } else {
        // 备用方案：使用OrderRef
        strncpy(trade_info.order_id, ctp_trade->OrderRef, sizeof(trade_info.order_id) - 1);
    }

    // 合约信息
    strncpy(trade_info.symbol, ctp_trade->InstrumentID, sizeof(trade_info.symbol) - 1);
    strncpy(trade_info.exchange, ctp_trade->ExchangeID, sizeof(trade_info.exchange) - 1);

    // 买卖方向
    trade_info.direction = (ctp_trade->Direction == THOST_FTDC_D_Buy) ?
        OrderDirection::BUY : OrderDirection::SELL;

    // 开平标志
    switch (ctp_trade->OffsetFlag) {
        case THOST_FTDC_OF_Open:
            trade_info.offset = OffsetFlag::OPEN;
            break;
        case THOST_FTDC_OF_Close:
            trade_info.offset = OffsetFlag::CLOSE;
            break;
        case THOST_FTDC_OF_CloseToday:
            trade_info.offset = OffsetFlag::CLOSE_TODAY;
            break;
        case THOST_FTDC_OF_CloseYesterday:
            trade_info.offset = OffsetFlag::CLOSE_YESTERDAY;
            break;
        default:
            trade_info.offset = OffsetFlag::OPEN;
    }

    trade_info.price = ctp_trade->Price;
    trade_info.volume = ctp_trade->Volume;

    // 成交时间（转换为纳秒时间戳）
    trade_info.trade_time = std::chrono::system_clock::now().time_since_epoch().count();
}

void CTPTDPlugin::ConvertPosition(CThostFtdcInvestorPositionField* ctp_pos, PositionInfo& pos_info) {
    if (!ctp_pos) return;

    // 合约信息
    strncpy(pos_info.symbol, ctp_pos->InstrumentID, sizeof(pos_info.symbol) - 1);
    strncpy(pos_info.exchange, ctp_pos->ExchangeID, sizeof(pos_info.exchange) - 1);

    // 持仓方向
    pos_info.direction = (ctp_pos->PosiDirection == THOST_FTDC_PD_Long) ?
        OrderDirection::BUY : OrderDirection::SELL;

    // 持仓数量
    pos_info.volume = ctp_pos->Position;
    pos_info.today_volume = ctp_pos->TodayPosition;
    pos_info.yesterday_volume = ctp_pos->YdPosition;

    // 持仓均价
    if (ctp_pos->Position > 0) {
        pos_info.avg_price = ctp_pos->PositionCost / ctp_pos->Position;
    } else {
        pos_info.avg_price = 0.0;
    }

    // 持仓盈亏
    pos_info.position_profit = ctp_pos->PositionProfit;

    // 占用保证金
    pos_info.margin = ctp_pos->UseMargin;
}

void CTPTDPlugin::ConvertAccount(CThostFtdcTradingAccountField* ctp_account, AccountInfo& account_info) {
    if (!ctp_account) return;

    // 账户ID
    strncpy(account_info.account_id, ctp_account->AccountID, sizeof(account_info.account_id) - 1);

    // 资金信息
    account_info.balance = ctp_account->Balance;                // 账户余额
    account_info.available = ctp_account->Available;            // 可用资金
    account_info.margin = ctp_account->CurrMargin;              // 占用保证金
    account_info.frozen_margin = ctp_account->FrozenMargin;     // 冻结保证金
    account_info.commission = ctp_account->Commission;          // 手续费
    account_info.close_profit = ctp_account->CloseProfit;       // 平仓盈亏
    account_info.position_profit = ctp_account->PositionProfit; // 持仓盈亏
}


// ==================== 订单缓存管理 ====================

void CTPTDPlugin::SaveOrder(const std::string& order_id, const OrderInfo& order_info) {
    std::lock_guard<std::mutex> lock(m_order_cache_mutex);
    m_order_cache[order_id] = order_info;
}

bool CTPTDPlugin::GetOrderFromCache(const std::string& order_id, OrderInfo& order_info) {
    std::lock_guard<std::mutex> lock(m_order_cache_mutex);
    auto it = m_order_cache.find(order_id);
    if (it != m_order_cache.end()) {
        order_info = it->second;
        return true;
    }
    return false;
}

} // namespace ctp
} // namespace plugin
} // namespace hft
