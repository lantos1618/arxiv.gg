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
  for (const width of [390, 1280]) {
   await page.setViewportSize({width, height:1000});
   for (const state of ['active','empty','new','error']) {
    await page.goto(new URL('/account?state='+state, base).href);
    assert.equal(await page.locator('h1').innerText(), 'Your account');
    assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'Horizontal overflow: '+width+' '+state);
    if (state === 'active') {
     assert.equal(await page.locator('.account-rotation').getAttribute('open'), null);
     await page.getByText('Replace API key', {exact:true}).first().click();
     assert.equal(await page.locator('[data-key-form]').evaluate(form => form.checkValidity()), false);
     await page.locator('[name=confirm_rotation]').check();
     assert.equal(await page.locator('[data-key-form]').evaluate(form => form.checkValidity()), true);
     await page.locator('.account-rotation summary').click();
    }
    if (state === 'empty') assert.equal(await page.getByRole('button',{name:'Create API key'}).count(), 1);
    if (state === 'error') {
     assert.match(await page.getByRole('alert').innerText(), /existing key has not been changed/);
     assert.notEqual(await page.locator('.account-rotation').getAttribute('open'), null);
    }
    if (state === 'new') {
     await page.getByRole('button',{name:'Copy API key',exact:true}).click();
     await page.waitForFunction(() => document.getElementById('account-copy-status').textContent === 'Copied to clipboard.');
     assert.equal(await page.evaluate(() => navigator.clipboard.readText()), 'arxivgg_fixture_only_not_a_real_key');
     await page.getByText('Authorization header',{exact:true}).click();
     await page.getByRole('button',{name:'Copy header'}).click();
     assert.equal(await page.evaluate(() => navigator.clipboard.readText()), 'Authorization: Bearer arxivgg_fixture_only_not_a_real_key');
    }
    await page.evaluate(() => {document.activeElement.blur(); window.scrollTo(0,0);});
    if (out) await page.screenshot({path:path.join(out,`account-${width}-${state}.png`),fullPage:true});
   }
  }
  // Prevent navigation after the real submit handler to inspect pending state.
  await page.goto(new URL('/account', base).href);
  await page.locator('.account-rotation summary').click();
  await page.locator('[name=confirm_rotation]').check();
  await page.locator('[data-key-form]').evaluate(form => {
   window.accountSubmissions = [];
   form.addEventListener('submit', event => {
    event.preventDefault();
    window.accountSubmissions.push(new URLSearchParams(new FormData(form)).toString());
   });
  });
  await page.getByRole('button',{name:'Replace API key',exact:true}).click();
  assert.equal(await page.locator('[data-key-form] button').isDisabled(), true);
  await page.locator('[data-key-form] button').evaluate(button => button.click());
  assert.deepEqual(await page.evaluate(() => window.accountSubmissions), ['confirm_rotation=rotate']);
  await page.evaluate(() => window.dispatchEvent(new PageTransitionEvent('pageshow', {persisted:true})));
  assert.equal(await page.locator('[data-key-form] button').isEnabled(), true);
  assert.deepEqual(errors,[]);
  console.log('PASS: account states at 390px/1280px, copy key/header, confirmation, duplicate-submit prevention, back navigation, no page errors');
 } finally { await browser.close(); }
})().catch(error => {console.error(error);process.exitCode=1;});
