'use strict';
// Real account templates, fixture keys only. No production account or database.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const {chromium} = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const base = new URL(process.env.ARXIV_TEST_URL || 'http://127.0.0.1:18081');
assert(['http:', 'https:'].includes(base.protocol) && ['127.0.0.1', 'localhost', '[::1]'].includes(base.hostname));
const out = process.env.ARXIV_TEST_OUTPUT_DIR;
if (out) fs.mkdirSync(out, {recursive:true});
(async () => {
 const browser = await chromium.launch({headless:true, executablePath:process.env.CHROMIUM_EXECUTABLE_PATH || undefined, args:['--no-sandbox']});
 try {
  const context = await browser.newContext({permissions:['clipboard-read','clipboard-write']});
  await context.route('**/*', route => new URL(route.request().url()).origin === base.origin ? route.continue() : route.abort());
  const page = await context.newPage();
  const errors = [];
  page.on('pageerror', error => errors.push(String(error)));
  const screenshot = async name => {
   await page.evaluate(() => {document.activeElement.blur(); window.scrollTo(0,0);});
   if (out) await page.screenshot({path:path.join(out,name),fullPage:true});
  };
  for (const width of [320,390,768,1280]) {
   await page.setViewportSize({width,height:900});
   for (const state of ['active','empty','new','error']) {
    await page.goto(new URL('/account?state='+state, base).href);
    assert.equal(await page.getByRole('heading',{name:'API keys',exact:true}).isVisible(),true);
    assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'Horizontal overflow: '+width+' '+state);
    if (state === 'active') {
     assert.equal(await page.locator('#rotation-dialog').isVisible(), false);
     await page.getByRole('button',{name:'Rotate key',exact:true}).click();
     assert.equal(await page.locator('#rotation-dialog').isVisible(),true);
     assert.equal(await page.locator('[data-key-form]').evaluate(form => form.checkValidity()),false);
     await page.locator('[name=confirm_rotation]').check();
     assert.equal(await page.locator('[data-key-form]').evaluate(form => form.checkValidity()),true);
     if ([390,1280].includes(width)) await screenshot(`account-${width}-rotation.png`);
     await page.getByRole('button',{name:'Cancel',exact:true}).click();
     assert.equal(await page.locator('#rotation-dialog').isVisible(),false);
     await page.getByRole('button',{name:'Rotate key',exact:true}).click();
     assert.equal(await page.locator('[name=confirm_rotation]').isChecked(),false);
     await page.keyboard.press('Escape');
     assert.equal(await page.locator('#rotation-dialog').isVisible(),false);
    }
    if (state === 'empty') assert.equal(await page.getByRole('button',{name:'Create API key'}).count(),1);
    if (state === 'error') assert.match(await page.getByRole('alert').innerText(),/existing key has not been changed/);
    if (state === 'new') {
     assert.equal(await page.locator('#account-api-key').getAttribute('type'),'password');
     await page.getByRole('button',{name:'Reveal',exact:true}).click();
     assert.equal(await page.locator('#account-api-key').getAttribute('type'),'text');
     await page.getByRole('button',{name:'Hide',exact:true}).click();
     await page.getByRole('button',{name:'Copy API key',exact:true}).click();
     await page.waitForFunction(() => document.getElementById('account-copy-status').textContent === 'Copied to clipboard.');
     assert.equal(await page.evaluate(() => navigator.clipboard.readText()),'arxivgg_fixture_only_not_a_real_key');
     await page.getByText('Authorization header',{exact:true}).click();
     await page.getByRole('button',{name:'Copy header'}).click();
     assert.equal(await page.evaluate(() => navigator.clipboard.readText()),'Authorization: Bearer arxivgg_fixture_only_not_a_real_key');
     await page.getByText('Authorization header',{exact:true}).click();
    }
    if ([390,1280].includes(width)) await screenshot(`account-${width}-${state}.png`);
   }
  }
  await page.goto(new URL('/account',base).href);
  await page.getByRole('link',{name:'Profile',exact:true}).click();
  await page.locator('#profile').waitFor({state:'visible'});
  assert.equal(await page.locator('#api-keys').isVisible(),false);
  await page.getByRole('link',{name:'Reading',exact:true}).click();
  await page.locator('#reading-history').waitFor({state:'visible'});
  await page.goBack();
  await page.locator('#profile').waitFor({state:'visible'});
  await page.reload();
  await page.locator('#profile').waitFor({state:'visible'});
  await page.getByRole('link',{name:'API keys',exact:true}).click();
  await page.getByRole('button',{name:'Rotate key',exact:true}).click();
  await page.locator('[name=confirm_rotation]').check();
  await page.locator('[data-key-form]').evaluate(form => {
   window.accountSubmissions=[];
   form.addEventListener('submit',event => {
    event.preventDefault();
    window.accountSubmissions.push(new URLSearchParams(new FormData(form)).toString());
   });
  });
  await page.getByRole('button',{name:'Rotate API key',exact:true}).click();
  assert.equal(await page.locator('[data-key-form] button[type=submit]').isDisabled(),true);
  await page.locator('[data-key-form] button[type=submit]').evaluate(button => button.click());
  assert.deepEqual(await page.evaluate(() => window.accountSubmissions),['confirm_rotation=rotate']);
  await page.evaluate(() => window.dispatchEvent(new PageTransitionEvent('pageshow',{persisted:true})));
  assert.equal(await page.locator('[data-key-form] button[type=submit]').isEnabled(),true);
  assert.deepEqual(errors,[]);
  const noJS=await browser.newContext({javaScriptEnabled:false});
  const fallback=await noJS.newPage();
  await fallback.goto(new URL('/account',base).href);
  await fallback.locator('#rotation-fallback summary').click();
  assert.equal(await fallback.getByRole('button',{name:'Rotate API key',exact:true}).isVisible(),true);
  await noJS.close();
  console.log('PASS: all key states at 320/390/768/1280px, reveal/copy, dialog confirmation/cancel/Escape, settings navigation/history, duplicate-submit prevention, no-JS fallback, no page errors');
 } finally { await browser.close(); }
})().catch(error => {console.error(error);process.exitCode=1;});
