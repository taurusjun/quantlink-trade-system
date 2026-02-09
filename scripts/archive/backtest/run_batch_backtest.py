#!/usr/bin/env python3
"""
run_batch_backtest.py - 批量回测脚本

用法:
  ./run_batch_backtest.py -c config.yaml -d dates.txt -o output/
  ./run_batch_backtest.py -c config.yaml --start 2026-01-01 --end 2026-01-31 -o output/

功能:
  - 支持日期列表文件或日期范围
  - 自动生成日期序列
  - 并行执行（可选）
  - 结果汇总
"""

import argparse
import subprocess
import sys
import os
from datetime import datetime, timedelta
from pathlib import Path
import json

def parse_args():
    parser = argparse.ArgumentParser(description='批量回测脚本')
    parser.add_argument('-c', '--config', required=True, help='配置文件路径')
    parser.add_argument('-d', '--dates', help='日期列表文件（每行一个日期）')
    parser.add_argument('--start', help='开始日期（YYYY-MM-DD）')
    parser.add_argument('--end', help='结束日期（YYYY-MM-DD）')
    parser.add_argument('-o', '--output', default='./backtest_results', help='输出目录')
    parser.add_argument('-j', '--jobs', type=int, default=1, help='并行任务数')
    parser.add_argument('--backtest-bin', default='./bin/backtest', help='回测可执行文件路径')
    return parser.parse_args()

def load_dates_from_file(filepath):
    """从文件加载日期列表"""
    dates = []
    with open(filepath, 'r') as f:
        for line in f:
            line = line.strip()
            if line and not line.startswith('#'):
                dates.append(line)
    return dates

def generate_date_range(start_date, end_date):
    """生成日期范围"""
    start = datetime.strptime(start_date, '%Y-%m-%d')
    end = datetime.strptime(end_date, '%Y-%m-%d')

    dates = []
    current = start
    while current <= end:
        dates.append(current.strftime('%Y-%m-%d'))
        current += timedelta(days=1)

    return dates

def run_single_backtest(config, date, output_dir, backtest_bin):
    """运行单次回测"""
    print(f"\n{'='*60}")
    print(f"回测日期: {date}")
    print(f"{'='*60}")

    cmd = [
        backtest_bin,
        '-config', config,
        '-start-date', date,
        '-end-date', date,
        '-output', output_dir,
    ]

    try:
        result = subprocess.run(cmd, check=True, capture_output=False)
        return True, None
    except subprocess.CalledProcessError as e:
        return False, str(e)

def summarize_results(output_dir):
    """汇总结果"""
    print(f"\n{'='*60}")
    print("结果汇总")
    print(f"{'='*60}\n")

    # 查找所有 JSON 结果文件
    json_files = list(Path(output_dir).glob('backtest_result_*.json'))

    if not json_files:
        print("未找到结果文件")
        return

    total_pnl = 0
    total_trades = 0
    total_wins = 0
    results = []

    for json_file in sorted(json_files):
        try:
            with open(json_file, 'r') as f:
                result = json.load(f)
                results.append(result)
                total_pnl += result.get('TotalPNL', 0)
                total_trades += result.get('TotalTrades', 0)
                total_wins += result.get('WinTrades', 0)
        except Exception as e:
            print(f"警告: 无法读取 {json_file}: {e}")

    if results:
        avg_pnl = total_pnl / len(results)
        win_rate = total_wins / total_trades if total_trades > 0 else 0

        print(f"总天数:         {len(results)}")
        print(f"总收益:         {total_pnl:.2f}")
        print(f"平均日收益:     {avg_pnl:.2f}")
        print(f"总交易次数:     {total_trades}")
        print(f"总胜率:         {win_rate*100:.1f}%")
        print(f"\n{'='*60}")

def main():
    args = parse_args()

    # 确定日期列表
    if args.dates:
        dates = load_dates_from_file(args.dates)
    elif args.start and args.end:
        dates = generate_date_range(args.start, args.end)
    else:
        print("错误: 必须指定 --dates 或 --start/--end")
        sys.exit(1)

    print(f"{'='*60}")
    print(f"批量回测")
    print(f"{'='*60}")
    print(f"配置文件:   {args.config}")
    print(f"日期数量:   {len(dates)}")
    print(f"输出目录:   {args.output}")
    print(f"并行任务:   {args.jobs}")
    print(f"{'='*60}")

    # 确保输出目录存在
    os.makedirs(args.output, exist_ok=True)

    # 执行回测
    successes = 0
    failures = 0

    if args.jobs == 1:
        # 串行执行
        for date in dates:
            success, error = run_single_backtest(
                args.config, date, args.output, args.backtest_bin
            )
            if success:
                successes += 1
            else:
                failures += 1
                print(f"失败: {date} - {error}")
    else:
        # 并行执行（简单实现，可使用 multiprocessing 优化）
        print("注意: 并行执行功能待实现，当前串行执行")
        for date in dates:
            success, error = run_single_backtest(
                args.config, date, args.output, args.backtest_bin
            )
            if success:
                successes += 1
            else:
                failures += 1
                print(f"失败: {date} - {error}")

    # 汇总结果
    summarize_results(args.output)

    # 打印最终统计
    print(f"\n{'='*60}")
    print("批量回测完成")
    print(f"{'='*60}")
    print(f"成功: {successes}")
    print(f"失败: {failures}")
    print(f"{'='*60}\n")

    return 0 if failures == 0 else 1

if __name__ == '__main__':
    sys.exit(main())
