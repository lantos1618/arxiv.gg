#!/usr/bin/env python3
"""Load test for SSE connections."""
import asyncio
import aiohttp
import sys
import time

async def sse_client(session, url, client_id, results):
    """Single SSE client that stays connected."""
    try:
        async with session.get(url, timeout=aiohttp.ClientTimeout(total=60)) as resp:
            # Read until complete event (initial load done)
            async for line in resp.content:
                if b'"type":"complete"' in line:
                    results['subscribed'] += 1
                    break
            # Stay connected for real-time updates
            while True:
                try:
                    line = await asyncio.wait_for(resp.content.readline(), timeout=60)
                    if not line:
                        break
                except asyncio.TimeoutError:
                    break
    except Exception as e:
        results['errors'] += 1
        if results['errors'] <= 3:
            print(f"  Error: {e}")

async def check_stats(session):
    """Get current SSE connection count."""
    try:
        async with session.get('http://localhost/api/v1/stats', timeout=aiohttp.ClientTimeout(total=5)) as resp:
            data = await resp.json()
            return data.get('data', {})
    except Exception as e:
        return {'error': str(e)}

async def main(num_connections):
    results = {'subscribed': 0, 'errors': 0}
    url = 'http://localhost/api/v1/papers/recent/stream?limit=1'

    connector = aiohttp.TCPConnector(limit=num_connections + 100)
    timeout = aiohttp.ClientTimeout(total=120)
    async with aiohttp.ClientSession(connector=connector, timeout=timeout) as session:
        # Start all connections
        print(f"Starting {num_connections} SSE connections...")
        start = time.time()

        tasks = [
            asyncio.create_task(sse_client(session, url, i, results))
            for i in range(num_connections)
        ]

        # Wait for connections to subscribe (with progress)
        for i in range(10):
            await asyncio.sleep(1)
            stats = await check_stats(session)
            sse = stats.get('SSEConnections', '?')
            print(f"  {i+1}s: subscribed={results['subscribed']}, server={sse}, errors={results['errors']}")
            if results['subscribed'] + results['errors'] >= num_connections:
                break

        elapsed = time.time() - start
        stats = await check_stats(session)

        print(f"\n=== Final Results ({elapsed:.1f}s) ===")
        print(f"Subscribed:      {results['subscribed']}")
        print(f"Server SSE:      {stats.get('SSEConnections', '?')}")
        print(f"Errors:          {results['errors']}")

        # Cancel all tasks
        for t in tasks:
            t.cancel()
        await asyncio.gather(*tasks, return_exceptions=True)

if __name__ == '__main__':
    n = int(sys.argv[1]) if len(sys.argv) > 1 else 100
    asyncio.run(main(n))
