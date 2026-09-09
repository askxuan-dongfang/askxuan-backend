#!/usr/bin/env python3
"""Create reviewable reward examples via admin API. Publishing requires --publish.
REWARDS_ADMIN_TOKEN must belong to platform_super. Existing examples are reused,
never overwritten, refilled or republished. No fake users, points or entries.
"""
import argparse
import json
import os
import time
import urllib.request


def seed(base, token, publish=False):
    def api(path, payload=None):
        req = urllib.request.Request(base.rstrip('/') + '/api/v1/admin/marketing/rewards' + path,
            data=None if payload is None else json.dumps(payload).encode(),
            headers={'Authorization': 'Bearer ' + token, 'Content-Type': 'application/json'})
        result = json.load(urllib.request.urlopen(req, timeout=30))
        if result.get('code') != 0:
            raise RuntimeError(f"{path}: {result.get('message', 'request failed')}")
        return result['data']

    existing = []
    for page in range(1, 1001):
        batch = api('/campaigns?page=' + str(page))
        existing.extend(batch)
        if len(batch) < 20:
            break
    else:
        raise RuntimeError('Campaign inventory exceeds safe scan limit')
    now = int(time.time())
    examples = [
        dict(title='秋日好礼 · 平安香囊积分转盘', kind='wheel', prizeName='平安香囊礼袋',
             description='示例活动：一枚草木香囊与便携礼袋。轻轻一转，为日常添一份草木清香。',
             pointsCost=10, prizeValue=3900, budget=100000, prizeQuantity=20, capacity=200),
        dict(title='一期一礼 · 天然香珠手串大奖池', kind='pool', prizeName='天然香珠手串礼盒',
             description='示例活动：天然香珠手串一条，配礼盒与收纳袋。活动截止后，在所有有效参与码中等概率抽出一名中奖者。',
             pointsCost=20, prizeValue=12900, budget=18000, prizeQuantity=1, capacity=100),
    ]
    out = []
    for sample in examples:
        current = next((c for c in existing if c['title'] == sample['title']), None)
        if current is None:
            sample.update(image='', startsAt=now-60, endsAt=now+14*86400,
                rules='每人每期消耗页面标明的积分参与一次，一人一码。成功参与后无论是否中奖均不退回积分，失败不扣分，重复请求不重复扣分。奖品与中国大陆地区运费由平台预算承担，不按商品价值折算积分。大奖池到期自动开奖，不等满额。中奖后在“我的奖品”填写地址，由平台安排发货。功德值不受影响。')
            current = api('/campaigns', sample)
        if publish and current['status'] == 'draft':
            api('/campaigns/' + str(current['id']) + '/publish', {})
            current = api('/campaigns/' + str(current['id']))['campaign']
        out.append({k: current[k] for k in ['id', 'title', 'status', 'pointsCost', 'prizeQuantity', 'capacity', 'participantCount', 'endsAt']})
    return out


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--base', default='http://127.0.0.1:8080')
    parser.add_argument('--publish', action='store_true', help='Publish only after platform confirms real prizes and budget')
    args = parser.parse_args()
    print(json.dumps(seed(args.base, os.environ['REWARDS_ADMIN_TOKEN'], args.publish), ensure_ascii=False, indent=2))
