# Trading Log Analyzer Memory

## System Architecture (Java Migration)
- Java trader uses same SysV MWMR SHM architecture as Go trader
- Log locations: `deploy_java/log/YYYYMMDD/`
- Strategy log: `log.control.{sym1}.{sym2}.par.txt.{stratId}.{date}`
- nohup log: `nohup.control.{sym1}.{sym2}.par.txt.{stratId}.{date}` (identical content to log file)
- Counter bridge: `counter_bridge.{date}.log`
- MD feeder: `md_shm_feeder.{date}.log`
- Overview: `overview.{date}.log`

## Known Issues (2026-03-11)
- **CRITICAL: Bridge returns ORDER PRICE not FILL PRICE** in type=4 responses. CTP actual trade price differs from Bridge response price. Bridge sends `price=22440` but CTP filled at `22391`. See `patterns.md` for details.
- **CRITICAL: OID=1 (ag2608) filled 43min late** — order sent 13:30:14, filled 14:13:03. CTP OrderRef `000000000003` was stuck in exchange queue.
- **Counter bridge Chinese encoding garbled** — ErrorID=26 messages show mojibake. UTF-8/GBK encoding issue.
- **CTP disconnect at end of day** — reason=4097 (network read failure) on both MD and TD, normal for SimNow after 15:00.
- SPREAD-GUARD warning at startup is expected (leg2 prices not yet received).

## Baseline Statistics (2026-03-11, ag2606/ag2608 pair)
- MD ticks: ~39,974
- Total orders sent: ~139 (counter bridge OID 0-138)
- Total fills: 79 (ag2606 BUY:42, SELL:27; ag2608 BUY:1, SELL:9)
- Cancel requests: 70, Cancel confirmations: 60
- CTP cancel failed (ErrorID=26): 10 occurrences (order already fully filled)
- Strategy activated via Web at 13:28:46, END TIME exit at 14:57:00
- Final position: netpos1=15 (ag2606 long), netpos2=-15 (ag2608 short)
