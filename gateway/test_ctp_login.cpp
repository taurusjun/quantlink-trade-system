// CTP登录测试程序 - 验证SimNow账号
// 编译: clang++ -std=c++11 test_ctp_login.cpp -o test_ctp_login \
//           -Ithird_party/ctp/include \
//           third_party/ctp/thostmduserapi_se.framework/Versions/A/thostmduserapi_se \
//           -Wl,-rpath,third_party/ctp/thostmduserapi_se.framework/Versions/A

#include "ThostFtdcMdApi.h"
#include <iostream>
#include <thread>
#include <chrono>
#include <atomic>

class LoginTestSpi : public CThostFtdcMdSpi {
public:
    LoginTestSpi(const char* broker_id, const char* user_id, const char* password,
                 const char* app_id, const char* auth_code)
        : m_broker_id(broker_id)
        , m_user_id(user_id)
        , m_password(password)
        , m_app_id(app_id)
        , m_auth_code(auth_code)
    {}

    void SetAPI(CThostFtdcMdApi* api) { m_api = api; }

    void OnFrontConnected() override {
        std::cout << "✅ 连接CTP前置成功！" << std::endl;
        std::cout << "正在发送登录请求..." << std::endl;

        // 构造登录请求
        CThostFtdcReqUserLoginField req = {};
        strncpy(req.BrokerID, m_broker_id, sizeof(req.BrokerID) - 1);
        strncpy(req.UserID, m_user_id, sizeof(req.UserID) - 1);
        strncpy(req.Password, m_password, sizeof(req.Password) - 1);

        int ret = m_api->ReqUserLogin(&req, ++m_request_id);
        if (ret != 0) {
            std::cerr << "❌ 发送登录请求失败，错误码: " << ret << std::endl;
            m_should_exit = true;
        }
    }

    void OnFrontDisconnected(int nReason) override {
        std::cerr << "❌ CTP断开连接，原因代码: " << nReason << std::endl;
        std::cerr << "   常见原因:" << std::endl;
        std::cerr << "   - 网络问题" << std::endl;
        std::cerr << "   - 前置服务器维护" << std::endl;
        std::cerr << "   - 登录失败次数过多" << std::endl;
        m_should_exit = true;
    }

    void OnRspUserLogin(CThostFtdcRspUserLoginField* pRspUserLogin,
                        CThostFtdcRspInfoField* pRspInfo,
                        int nRequestID, bool bIsLast) override {
        if (pRspInfo && pRspInfo->ErrorID != 0) {
            std::cerr << "\n❌ 登录失败！" << std::endl;
            std::cerr << "   错误码: " << pRspInfo->ErrorID << std::endl;
            std::cerr << "   错误信息: " << pRspInfo->ErrorMsg << std::endl;
            std::cerr << "\n常见错误排查:" << std::endl;
            std::cerr << "   1. 检查BrokerID是否正确（应为: 9999）" << std::endl;
            std::cerr << "   2. 检查UserID和Password是否正确" << std::endl;
            std::cerr << "   3. 检查网络连接" << std::endl;
            std::cerr << "   4. 确认账号已在SimNow激活" << std::endl;
            m_login_success = false;
        } else {
            std::cout << "\n🎉 登录成功！" << std::endl;
            if (pRspUserLogin) {
                std::cout << "   交易日: " << pRspUserLogin->TradingDay << std::endl;
                std::cout << "   登录时间: " << pRspUserLogin->LoginTime << std::endl;
                std::cout << "   前置版本: " << pRspUserLogin->FrontID << std::endl;
                std::cout << "   会话编号: " << pRspUserLogin->SessionID << std::endl;
            }
            std::cout << "\n✅ 账号验证通过，可以开始开发了！" << std::endl;
            m_login_success = true;
        }
        m_should_exit = true;
    }

    void OnRspError(CThostFtdcRspInfoField* pRspInfo, int nRequestID, bool bIsLast) override {
        if (pRspInfo) {
            std::cerr << "\n❌ 收到错误响应" << std::endl;
            std::cerr << "   错误码: " << pRspInfo->ErrorID << std::endl;
            std::cerr << "   错误信息: " << pRspInfo->ErrorMsg << std::endl;
        }
        m_should_exit = true;
    }

    bool ShouldExit() const { return m_should_exit; }
    bool IsLoginSuccess() const { return m_login_success; }

private:
    CThostFtdcMdApi* m_api = nullptr;
    const char* m_broker_id;
    const char* m_user_id;
    const char* m_password;
    const char* m_app_id;
    const char* m_auth_code;
    int m_request_id = 0;
    std::atomic<bool> m_should_exit{false};
    bool m_login_success = false;
};

int main(int argc, char* argv[]) {
    std::cout << R"(
╔═══════════════════════════════════════════════════════╗
║         CTP登录测试 - SimNow账号验证               ║
╚═══════════════════════════════════════════════════════╝
)" << std::endl;

    // 从命令行读取账号信息
    std::string user_id, password;

    if (argc >= 3) {
        user_id = argv[1];
        password = argv[2];
    } else {
        std::cout << "请输入您的SimNow账号信息：" << std::endl;
        std::cout << "UserID: ";
        std::getline(std::cin, user_id);
        std::cout << "Password: ";
        std::getline(std::cin, password);
    }

    if (user_id.empty() || password.empty()) {
        std::cerr << "❌ 用户名和密码不能为空！" << std::endl;
        return 1;
    }

    // SimNow 7x24环境配置
    const char* front_addr = "tcp://182.254.243.31:40011";
    const char* broker_id = "9999";
    const char* app_id = "simnow_client_test";
    const char* auth_code = "0000000000000000";

    std::cout << "\n配置信息：" << std::endl;
    std::cout << "  前置地址: " << front_addr << std::endl;
    std::cout << "  BrokerID: " << broker_id << std::endl;
    std::cout << "  UserID: " << user_id << std::endl;
    std::cout << "  AppID: " << app_id << std::endl;
    std::cout << "\n正在连接..." << std::endl;

    try {
        // 创建API实例
        CThostFtdcMdApi* api = CThostFtdcMdApi::CreateFtdcMdApi("./ctp_test_flow/");

        // 创建回调处理
        LoginTestSpi spi(broker_id, user_id.c_str(), password.c_str(), app_id, auth_code);
        spi.SetAPI(api);
        api->RegisterSpi(&spi);

        // 连接前置
        api->RegisterFront(const_cast<char*>(front_addr));
        api->Init();

        // 等待结果（最多30秒）
        int wait_count = 0;
        while (!spi.ShouldExit() && wait_count < 300) {
            std::this_thread::sleep_for(std::chrono::milliseconds(100));
            wait_count++;
        }

        if (wait_count >= 300) {
            std::cerr << "\n❌ 连接超时（30秒）" << std::endl;
            std::cerr << "请检查：" << std::endl;
            std::cerr << "  1. 网络连接是否正常" << std::endl;
            std::cerr << "  2. 防火墙是否阻止了连接" << std::endl;
            std::cerr << "  3. SimNow服务器是否在维护" << std::endl;
        }

        // 释放资源
        api->Release();

        std::cout << "\n测试结束。" << std::endl;
        return spi.IsLoginSuccess() ? 0 : 1;

    } catch (const std::exception& e) {
        std::cerr << "❌ 异常: " << e.what() << std::endl;
        return 1;
    }
}
