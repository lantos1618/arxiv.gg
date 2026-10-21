'use strict';

// Browser regressions for F05-F08. Run against an isolated, seeded local server:
// ARXIV_TEST_URL=http://127.0.0.1:8080 node tools/test_graph_ui.cjs
// The seeded paper must have HasEmbedding=true to exercise the related-work map.
// Optional: ARXIV_TEST_PAPER, ARXIV_TEST_AUTHOR, PLAYWRIGHT_MODULE,
// CHROMIUM_EXECUTABLE_PATH, D3_SCRIPT_PATH. All API graph data is intercepted.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const base = new URL(process.env.ARXIV_TEST_URL || 'http://127.0.0.1:8080');
assert(['127.0.0.1', 'localhost', '[::1]'].includes(base.hostname), 'Use an isolated loopback test server');
const paperID = process.env.ARXIV_TEST_PAPER || '1706.03762';
const author = process.env.ARXIV_TEST_AUTHOR || 'Ashish Vaswani';
const paperURL = new URL('/abs/' + paperID, base).href;
const authorURL = new URL('/author/' + encodeURIComponent(author), base).href;
const failures = [], checks = [];
const citationData = {
    nodes: [
        { id: paperID, title: 'Test paper', year: 2017, citations: 5, cached: true },
        { id: '1801.00001', title: 'Test reference', year: 2018, citations: 2, cached: true },
    ],
    edges: [{ source: paperID, target: '1801.00001' }],
};
const authorData = name => ({ success: true, data: { nodes: [{ id: 'fixture', name, paperCount: 1, isCenter: true }], edges: [] } });
const json = (route, data, status = 200) => route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(data) });
async function check(name, run) {
    await run();
    checks.push(name);
    console.log('PASS ' + name);
}
async function nextFrame(page) {
    await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
}
async function contextFor(browser, motion) {
    const context = await browser.newContext({ viewport: { width: 1365, height: 1000 }, reducedMotion: motion, serviceWorkers: 'block' });
    await context.route('**/*', async route => {
        const request = route.request();
        const url = new URL(request.url());
        if (!['GET', 'HEAD', 'OPTIONS'].includes(request.method())) {
            failures.push('Unexpected mutation: ' + request.method() + ' ' + url.pathname);
            return route.abort();
        }
        if (url.hostname === 'cdn.jsdelivr.net' && url.pathname.includes('/d3@')) {
            if (process.env.D3_SCRIPT_PATH) return route.fulfill({ contentType: 'text/javascript', headers: { 'access-control-allow-origin': '*' }, body: fs.readFileSync(process.env.D3_SCRIPT_PATH) });
            return route.continue();
        }
        if (url.origin !== base.origin) return route.abort();
        if (url.pathname === '/paper/' + paperID + '/graph') return json(route, citationData);
        if (url.pathname === '/api/v1/papers/' + paperID + '/similar') {
            return json(route, { success: true, data: { model: 'Qwen/Qwen3-Embedding-8B', results: [
                { paper: { ID: '1801.00001', Title: 'Related test paper', Categories: 'cs.CL' }, similarity: 0.9 },
            ] } });
        }
        if (url.pathname.startsWith('/api/v1/authors/') && !url.pathname.endsWith('/graph')) {
            return json(route, { success: true, data: {} });
        }
        return route.continue();
    });
    context.on('page', page => page.on('pageerror', error => failures.push(error.message)));
    return context;
}
async function graphFits(page, fullscreen = false) {
    await page.waitForFunction(fullscreen => {
        const c = document.getElementById('graph-container'), s = c.querySelector('svg');
        if (!s || c.classList.contains('fullscreen') !== fullscreen) return false;
        const cr = c.getBoundingClientRect(), sr = s.getBoundingClientRect();
        const nodes = Array.from(c.querySelectorAll('.graph-node'));
        const centerX = nodes.reduce((sum, n) => sum + Number(n.getAttribute('cx')), 0) / nodes.length;
        const centerY = nodes.reduce((sum, n) => sum + Number(n.getAttribute('cy')), 0) / nodes.length;
        return Number(s.getAttribute('width')) === c.clientWidth && Number(s.getAttribute('height')) === c.clientHeight &&
            sr.width <= c.clientWidth + 1 && sr.height <= c.clientHeight + 1 && sr.bottom <= cr.bottom + 1 &&
            Math.abs(centerX - c.clientWidth / 2) < 3 && Math.abs(centerY - c.clientHeight / 2) < 3 &&
            (!fullscreen || (Math.abs(cr.height - innerHeight) < 2 && cr.top === 0));
    }, fullscreen);
    assert.equal(await page.locator('#graph-container').evaluate(c => getComputedStyle(c).overflow), 'hidden');
    if (!fullscreen) {
        assert(await page.evaluate(() => document.getElementById('related-work-heading').getBoundingClientRect().top >= document.getElementById('graph-container').getBoundingClientRect().bottom));
    }
}
async function testCitation(browser, motion) {
    const context = await contextFor(browser, motion);
    const page = await context.newPage();
    page.setDefaultTimeout(10000);
    await page.goto(paperURL, { waitUntil: 'domcontentloaded' });
    await page.locator('#graph-container .graph-node').first().waitFor();
    await check(motion + ': initial citation graph fits and is centered', () => graphFits(page));
    await check(motion + ': keyboard fullscreen then Escape restores dimensions', async () => {
        await page.locator('#fullscreen-btn').focus();
        await page.keyboard.press('Enter');
        await graphFits(page, true);
        assert.equal(await page.locator('#fullscreen-btn').getAttribute('aria-pressed'), 'true');
        await page.keyboard.press('Escape');
        await graphFits(page);
        assert.equal(await page.locator('#fullscreen-btn').getAttribute('aria-pressed'), 'false');
    });
    await check(motion + ': Exit button and rapid fullscreen toggles restore dimensions', async () => {
        await page.locator('#fullscreen-btn').click();
        await graphFits(page, true);
        await page.locator('#fullscreen-btn').click();
        await graphFits(page);
        await page.evaluate(async () => {
            for (let i = 0; i < 10; i++) {
                document.getElementById('fullscreen-btn').click();
                await new Promise(requestAnimationFrame);
            }
        });
        await graphFits(page);
    });
    await check(motion + ': desktop to mobile and rapid viewport changes resize and recenter', async () => {
        for (const width of [390, 1200, 500, 1365, 390]) await page.setViewportSize({ width, height: 844 });
        await graphFits(page);
        assert.equal(await page.locator('#graph-container').evaluate(c => c.clientHeight), 298);
    });
    await check(motion + ': mobile fullscreen fills viewport and resizes while open', async () => {
        await page.locator('#fullscreen-btn').click();
        await graphFits(page, true);
        await page.setViewportSize({ width: 844, height: 390 });
        await graphFits(page, true);
        await page.setViewportSize({ width: 390, height: 844 });
        await graphFits(page, true);
        await page.keyboard.press('Escape');
        await graphFits(page);
    });
    await check(motion + ': Reset still clears pan and zoom after resizing', async () => {
        const graph = page.locator('#graph-container > svg');
        await graph.scrollIntoViewIfNeeded();
        const box = await graph.boundingBox();
        await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
        await page.mouse.wheel(0, -300);
        await page.waitForFunction(() => document.querySelector('#graph-container > svg').__zoom.k > 1);
        await page.locator('#reset-btn').click();
        await page.waitForFunction(() => {
            const z = document.querySelector('#graph-container > svg').__zoom;
            return z.k === 1 && z.x === 0 && z.y === 0;
        });
    });
    await check(motion + ': tooltip is outside the clipped graph and still shows node details', async () => {
        await page.locator('#graph-container .graph-node').first().dispatchEvent('mouseover');
        assert.equal(await page.locator('#tooltip').isVisible(), true);
        assert.match(await page.locator('#tooltip').innerText(), /Test paper/);
        assert.equal(await page.locator('#graph-container #tooltip').count(), 0);
        await page.locator('#graph-container .graph-node').first().dispatchEvent('mouseout');
        assert.equal(await page.locator('#tooltip').isVisible(), false);
    });
    await check(motion + ': related map selection and keyboard controls survive graph resizing', async () => {
        const select = page.locator('#related-papers-list button').first();
        await select.waitFor();
        await select.focus();
        await page.keyboard.press('Enter');
        assert.equal(await select.getAttribute('aria-pressed'), 'true');
        await page.setViewportSize({ width: 1365, height: 1000 });
        await graphFits(page);
        await page.waitForFunction(() => document.querySelectorAll('#semantic-map-container .semantic-map-node.selected').length === 1);
        assert.equal(await select.getAttribute('aria-pressed'), 'true');
        assert.match(await page.locator('#semantic-selected').innerText(), /Related test paper/);
    });
    await context.close();
}
async function testAuthor(browser) {
    const context = await contextFor(browser, 'reduce');
    // The race fault ignores transport cancellation so request identity is tested
    // even if JSON parsing/another transport completes after abort was requested.
    await context.addInitScript(() => {
        const nativeFetch = window.fetch.bind(window);
        window.__graphSignals = [];
        window.fetch = (url, options) => {
            if (String(url).includes('/api/v1/authors/graph?') && window.__ignoreGraphAbort) {
                window.__graphSignals.push(options.signal);
                return nativeFetch(url, { ...options, signal: undefined });
            }
            return nativeFetch(url, options);
        };
    });
    const page = await context.newPage();
    page.setDefaultTimeout(10000);
    let mode = 'http-error';
    const pending = [];
    await page.route('**/api/v1/authors/graph?*', route => {
        if (mode === 'defer') { pending.push(route); return; }
        if (mode === 'http-error') return json(route, { success: false }, 503);
        if (mode === 'api-error') return json(route, { success: false });
        if (mode === 'network-error') return route.abort('failed');
        if (mode === 'invalid-json') return route.fulfill({ contentType: 'application/json', body: '{' });
        return json(route, authorData(mode));
    });
    const status = page.locator('#author-graph-status'), retry = page.locator('#author-graph-retry');
    const depth = page.locator('#depth-toggle'), graph = page.locator('#collab-graph');
    async function failureVisible(previous) {
        await retry.waitFor({ state: 'visible' });
        assert.match(await status.innerText(), /Could not load the collaboration network/);
        assert.equal(await page.locator('#graph-container').getAttribute('aria-busy'), 'false');
        if (previous) assert.match(await graph.textContent(), new RegExp(previous));
    }
    async function rendered(name) {
        await page.waitForFunction(name => document.getElementById('collab-graph').textContent.includes(name), name);
        assert.equal(await retry.isVisible(), false);
        assert.equal(await page.locator('#graph-container').getAttribute('aria-busy'), 'false');
    }
    async function deferredCount(count) {
        for (let i = 0; i < 100 && pending.length < count; i++) await new Promise(resolve => setTimeout(resolve, 10));
        assert.equal(pending.length, count, 'Expected intercepted graph requests');
    }
    await page.goto(authorURL, { waitUntil: 'domcontentloaded' });
    await check('author: initial HTTP error is visible, accessible and retry succeeds', async () => {
        await failureVisible();
        assert.equal(await status.getAttribute('role'), 'status');
        mode = 'Initial graph';
        await retry.focus();
        await page.keyboard.press('Enter');
        await rendered(mode);
    });
    await check('author: failed depth change preserves graph, retry uses selected depth', async () => {
        mode = 'api-error';
        await depth.check();
        await failureVisible('Initial graph');
        assert.match(await status.innerText(), /previous graph is still shown/);
        mode = 'Depth two';
        const requested = page.waitForRequest(r => r.url().includes('/authors/graph?') && new URL(r.url()).searchParams.get('depth') === '2');
        await retry.click();
        await requested;
        await rendered(mode);
        assert.equal(await depth.isChecked(), true);
    });
    await check('author: malformed JSON and network errors preserve valid graph', async () => {
        mode = 'invalid-json';
        await depth.uncheck();
        await failureVisible('Depth two');
        mode = 'network-error';
        await retry.click();
        await failureVisible('Depth two');
        mode = 'Recovered';
        await retry.click();
        await rendered(mode);
    });
    await check('author: older success cannot overwrite latest selected depth', async () => {
        await page.evaluate(() => { window.__ignoreGraphAbort = true; });
        mode = 'defer';
        await depth.check();
        await deferredCount(1);
        await depth.uncheck();
        await deferredCount(2);
        assert.equal(await page.evaluate(() => window.__graphSignals[0].aborted), true, 'Superseded request must be cancelled');
        await json(pending[1], authorData('Newest depth 1'));
        await rendered('Newest depth 1');
        await json(pending[0], authorData('Stale depth 2'));
        await nextFrame(page);
        assert.match(await graph.textContent(), /Newest depth 1/);
        assert.equal(await depth.isChecked(), false);
        pending.length = 0;
    });
    await check('author: stale failure cannot replace latest success with an error', async () => {
        await depth.check();
        await deferredCount(1);
        await depth.uncheck();
        await deferredCount(2);
        await json(pending[1], authorData('Current graph'));
        await rendered('Current graph');
        await json(pending[0], { success: false }, 503);
        await nextFrame(page);
        await rendered('Current graph');
        assert.match(await status.innerText(), /Showing direct collaborators/);
        pending.length = 0;
    });
    await check('author: native transport abort keeps newest graph visible', async () => {
        await page.evaluate(() => { window.__ignoreGraphAbort = false; });
        await depth.check();
        await deferredCount(1);
        const aborted = page.waitForEvent('requestfailed', request => request.url().includes('/authors/graph?') && request.failure()?.errorText.includes('ABORTED'));
        mode = 'After abort';
        await depth.uncheck();
        await aborted;
        await rendered(mode);
        // An aborted Playwright route can still be fulfilled; the browser ignores it.
        await json(pending[0], authorData('Aborted graph'));
        await nextFrame(page);
        await rendered(mode);
    });
    await context.close();
}
(async () => {
    const browser = await chromium.launch({ headless: true, executablePath: process.env.CHROMIUM_EXECUTABLE_PATH || undefined, args: ['--no-sandbox'] });
    try {
        await testCitation(browser, 'reduce');
        await testCitation(browser, 'no-preference');
        await testAuthor(browser);
        assert.deepEqual(failures, [], 'No uncaught browser errors or mutation requests');
        console.log(JSON.stringify({ passed: checks.length, failures }));
    } finally {
        await browser.close();
    }
})().catch(error => { console.error(error); process.exitCode = 1; });
