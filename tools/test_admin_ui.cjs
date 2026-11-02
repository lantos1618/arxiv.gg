'use strict';
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const base=new URL(process.env.ARXIV_TEST_URL||'http://127.0.0.1:18081');
assert(['127.0.0.1','localhost','[::1]'].includes(base.hostname));
(async()=>{
 const browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE_PATH||undefined,args:['--no-sandbox']});
 try {
  const page=await browser.newPage();const errors=[];page.on('pageerror',e=>errors.push(String(e)));
  await page.route('**/*',route=>new URL(route.request().url()).origin===base.origin?route.continue():route.abort());
  for(const width of [320,390,768,1280]){
   await page.setViewportSize({width,height:950});
   for(const route of ['/admin','/admin/users','/admin/audit','/admin/feedback']){
    await page.goto(new URL(route,base).href);
    assert.equal(await page.locator('h1').count(),1);
    assert.equal(await page.locator('.admin-tabs [aria-current=page]').getAttribute('href'),route);
    assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),`Overflow ${route} ${width}`);
    for(const detail of await page.locator('.admin-details').all()){
     await detail.locator(':scope > summary').click();
     assert.equal(await detail.getAttribute('open'),'');
     assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),`Expanded overflow ${route} ${width}`);
     await detail.locator(':scope > summary').click();
    }
    if(process.env.ARXIV_TEST_OUTPUT_DIR&&[390,1280].includes(width)){
     fs.mkdirSync(process.env.ARXIV_TEST_OUTPUT_DIR,{recursive:true});
     await page.screenshot({path:path.join(process.env.ARXIV_TEST_OUTPUT_DIR,route.replaceAll('/','-')+'-'+width+'.png'),fullPage:true});
    }
   }
  }
  assert.deepEqual(errors,[]);
  console.log('PASS: admin navigation, all four screens, expandable details, desktop/mobile layout, no page errors');
 }finally{await browser.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
