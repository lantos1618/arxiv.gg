'use strict';

// Search UI regressions against TestUIBrowserFixtureServer; no database is opened.
// ARXIV_TEST_URL=http://127.0.0.1:18081 node tools/test_search_ui.cjs
// Optional: PLAYWRIGHT_MODULE, CHROMIUM_EXECUTABLE_PATH, ARXIV_TEST_OUTPUT_DIR.
// Search API responses are controlled fixtures. Artifacts are written only when
// ARXIV_TEST_OUTPUT_DIR is set; the test summary is always printed to stdout.
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const {chromium} = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const out = process.env.ARXIV_TEST_OUTPUT_DIR ? path.resolve(process.env.ARXIV_TEST_OUTPUT_DIR) : null;
const target = new URL(process.env.ARXIV_TEST_URL || 'http://127.0.0.1:18081');
assert(['http:', 'https:'].includes(target.protocol) && ['127.0.0.1', 'localhost', '[::1]'].includes(target.hostname), 'Use an isolated loopback test server');
const base = target.origin;
if (out) fs.mkdirSync(out, {recursive:true});
const report = {base, method:'Real Chromium, loopback template fixture, controlled GET search streams, no database', checks:[], requests:[], pageErrors:[], startedAt:new Date().toISOString()};
const saveReport = () => { if (out) fs.writeFileSync(path.join(out,'results.json'),JSON.stringify(report,null,2)+'\n'); };
const screenshot = (page, name) => out ? page.screenshot({path:path.join(out,name)}) : Promise.resolve();
const streamBody = data => data.map(value=>'data: '+JSON.stringify(value)+'\n\n').join('');
(async()=>{
 const browser = await chromium.launch({executablePath:process.env.CHROMIUM_EXECUTABLE_PATH || undefined,headless:true,args:['--no-sandbox']});
 try {
 const context = await browser.newContext({viewport:{width:1365,height:1000}});
 let holdQuery = '';
 const pending = [];
 await context.route('**/*', async route => {
  const url = new URL(route.request().url());
  if (url.origin !== base) return route.abort();
  if (!['GET','HEAD'].includes(route.request().method())) throw new Error('Unexpected mutation '+route.request().url());
  if (url.pathname === '/api/v1/search/stream') {
   const row = Object.fromEntries(url.searchParams); report.requests.push(row);
   const fulfill = ()=>route.fulfill({status:200,contentType:'text/event-stream',body:streamBody([
    {type:'start'},
    {type:'result',paper:{ID:'2501.00001',Title:row.q+' '+row.mode+' fixture result',Authors:'Test Author',Categories:row.category||'cs.AI'}},
    {type:'complete',count:1,mode:row.mode}
   ])}).catch(()=>{});
   if (row.q === holdQuery) {pending.push(fulfill);return;}
   return fulfill();
  }
  if (url.pathname === '/api/v1/search/quick') return route.fulfill({status:200,contentType:'application/json',body:JSON.stringify({success:true,data:{papers:[],authors:[],count:0}})});
  if (url.pathname === '/abs/1706.03762' || url.pathname === '/abs/math/0309136' || url.pathname === '/abs/q-bio/0401004') return route.fulfill({status:200,contentType:'text/html',body:'<h1>Paper shortcut fixture</h1>'});
  return route.continue();
 });
 const page = await context.newPage();
 page.on('pageerror',err=>report.pageErrors.push(String(err)));
 const check = async (name,fn)=>{try{await fn();report.checks.push({name,status:'PASS'});}catch(err){report.checks.push({name,status:'FAIL',error:String(err)});throw err;}finally{saveReport();}};
 const searchURL = base+'/search?q=graph&mode=quick&category=cs.AI&limit=7';
 await page.goto(searchURL,{waitUntil:'domcontentloaded'});
 await check('Quick search retains category and limit',async()=>{
  await page.locator('#search-input').fill('attention');
  await page.locator('#btn-quick').click();
  await page.waitForFunction(()=>document.querySelector('#search-status').textContent.includes('1 quick results for "attention"'));
  assert.equal(report.requests.at(-1).category,'cs.AI'); assert.equal(report.requests.at(-1).limit,'7');
 });
 await check('Back restores input, results, status and category/limit',async()=>{
  await page.goBack();
  await page.waitForFunction(()=>document.querySelector('#search-status').textContent.includes('1 quick results for "graph"'));
  assert.equal(await page.locator('#search-input').inputValue(),'graph');
  assert.match(await page.locator('#paper-results').innerText(),/graph quick fixture result/);
  assert.equal(report.requests.at(-1).category,'cs.AI'); assert.equal(report.requests.at(-1).limit,'7');
  await screenshot(page,'back-restores-results.png');
 });
 await check('Forward restores input, results and status',async()=>{
  await page.goForward();
  await page.waitForFunction(()=>document.querySelector('#search-status').textContent.includes('1 quick results for "attention"'));
  assert.equal(await page.locator('#search-input').inputValue(),'attention');
  assert.match(await page.locator('#paper-results').innerText(),/attention quick fixture result/);
 });
 await check('Back and Forward restore semantic mode',async()=>{
  await page.locator('#search-input').fill('semantic idea'); await page.locator('#btn-semantic').click();
  await page.waitForFunction(()=>document.querySelector('#search-status').textContent.includes('1 Semantic results for "semantic idea"'));
  await page.goBack(); await page.waitForFunction(()=>document.querySelector('#search-status').textContent.includes('1 quick results for "attention"'));
  await page.goForward(); await page.waitForFunction(()=>document.querySelector('#search-status').textContent.includes('1 Semantic results for "semantic idea"'));
  assert.equal(await page.locator('#search-mode').inputValue(),'semantic');
  assert.equal(report.requests.at(-1).mode,'semantic');
 });
 await check('Back cancels an outstanding stream before it can overwrite restored results',async()=>{
  holdQuery='slow';
  await page.locator('#search-input').fill('slow'); await page.locator('#btn-quick').click();
  await page.waitForFunction(()=>document.querySelector('#search-status').textContent==='Searching...');
  await new Promise(resolve=>setTimeout(resolve,100));
  assert.equal(pending.length,1);
  await page.goBack();
  await page.waitForFunction(()=>document.querySelector('#search-status').textContent.includes('1 Semantic results for "semantic idea"'));
  await pending[0]();
  await new Promise(resolve=>setTimeout(resolve,100));
  assert.doesNotMatch(await page.locator('#paper-results').innerText(),/slow/);
  assert.equal(await page.locator('#search-input').inputValue(),'semantic idea');
  holdQuery='';
 });
 for (const name of ['search','index']) {
  await page.goto(name==='search'?searchURL:base+'/',{waitUntil:'domcontentloaded'});
  for (const trigger of ['quick','semantic','deep','Enter']) for (const input of ['', '   ']) {
   await check(name+' '+trigger+' validates '+(input?'whitespace':'empty')+' input',async()=>{
    const before = report.requests.length;
    await page.locator('#search-input').fill(input);
    if (trigger==='Enter') await page.locator('#search-input').press('Enter');
    else await page.locator('#btn-'+trigger).click();
    await page.waitForFunction(()=>document.querySelector('#search-status')?.textContent.includes('Enter a paper ID'));
    assert.equal(await page.evaluate(()=>document.activeElement.id),'search-input');
    assert.equal(report.requests.length,before);
   });
  }
  await screenshot(page,name+'-empty-validation.png');
 }
 for (const [value,id] of [['1706.03762v7','1706.03762'],['math.GT/0309136v2','math/0309136'],['https://arxiv.org/pdf/math.GT/0309136v2.pdf?download=1','math/0309136'],['Q-BIO.BM/0401004v1','q-bio/0401004']]) {
  await check('Autocomplete paper shortcut: '+value,async()=>{
   await page.goto(base+'/',{waitUntil:'domcontentloaded'});
   await page.locator('#search-input').fill(value);
   const link=page.locator('#search-dropdown .dropdown-item');
   await link.waitFor({state:'visible'});
   assert.equal(await link.getAttribute('href'),'/abs/'+id);
   await page.locator('#search-input').press('ArrowDown');
   await page.keyboard.press('Enter');
   await page.waitForURL(base+'/abs/'+id);
  });
  await check('Result-page paper shortcut: '+value,async()=>{
   await page.goto(searchURL,{waitUntil:'domcontentloaded'});
   const before=report.requests.length;
   await page.locator('#search-input').fill(value); await page.locator('#btn-quick').click();
   await page.waitForURL(base+'/abs/'+id);
   assert.equal(report.requests.length,before);
   await page.goBack();
   await page.waitForURL(searchURL);
   assert.equal(await page.locator('#search-input').inputValue(),'graph');
  });
 }
 assert.deepEqual(report.pageErrors,[]);
 report.finishedAt=new Date().toISOString();saveReport();
 console.log(JSON.stringify({checks:report.checks.length,passed:report.checks.filter(x=>x.status==='PASS').length,failed:report.checks.filter(x=>x.status==='FAIL').length,pageErrors:report.pageErrors}));
 } finally {
  await browser.close();
 }
})().catch(err=>{report.fatal=String(err);saveReport();console.error(err);process.exitCode=1;});
