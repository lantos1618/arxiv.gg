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
     assert.equal(await page.locator('.key-table tbody tr').count(),2);
     for (const action of ['rotate','revoke']) {
      await page.locator(`[data-key-action="${action}"][data-key-id="key_fixture_two"]`).click();
      const dialog=page.locator('#key-editor');
      assert.equal(await dialog.isVisible(),true);
      assert.match(await dialog.locator('h3').innerText(),/Research script/);
      assert.equal(await dialog.locator('[name=key_id]').inputValue(),'key_fixture_two');
      assert.equal(await dialog.locator('form').evaluate(form => form.checkValidity()),false);
      await dialog.locator('[type=checkbox]').check();
      assert.equal(await dialog.locator('form').evaluate(form => form.checkValidity()),true);
      if ([390,1280].includes(width)) await screenshot(`account-${width}-${action}.png`);
      await dialog.getByRole('button',{name:'Cancel',exact:true}).click();
      assert.equal(await dialog.isVisible(),false);
      await page.locator(`[data-key-action="${action}"][data-key-id="key_fixture_two"]`).click();
      assert.equal(await dialog.locator('[type=checkbox]').isChecked(),false);
      await page.keyboard.press('Escape');
      assert.equal(await dialog.isVisible(),false);
     }
    }
    await page.locator('[data-open-create]').click();
    const create=page.locator('#key-editor');
    assert.equal(await create.locator('form').evaluate(form => form.checkValidity()),false);
    await create.getByRole('textbox',{name:'Key name'}).fill('Codex');
    assert.equal(await create.locator('form').evaluate(form => form.checkValidity()),true);
    assert.equal(await create.locator('form').getAttribute('action'),'/account/api-key/create');
    if ([390,1280].includes(width) && state==='active') await screenshot(`account-${width}-create.png`);
    await page.keyboard.press('Escape');
    assert.equal(await create.isVisible(),false);
    assert.equal(await page.locator('[data-open-create]').evaluate(node=>node===document.activeElement),true);
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
  for (const width of [320,1280]) {
   await page.setViewportSize({width,height:900});
   await page.goto(new URL('/account',base).href);
   await page.locator('.key-name strong').first().evaluate(node => { node.textContent='x'.repeat(120); });
   assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),'Long key name overflows table');
   const action=page.locator('[data-key-action=rotate]').first();
   await action.evaluate(node => { node.dataset.keyName='x'.repeat(120); });
   await action.click();
   assert(await page.locator('#key-editor').evaluate(node => node.scrollWidth <= node.clientWidth),'Long key name overflows dialog');
   await page.keyboard.press('Escape');
  }
  await page.goto(new URL('/account',base).href);
  assert.equal(await page.locator('#profile').isVisible(),true);
  assert.equal(await page.locator('#api-keys').isVisible(),true);
  await page.locator('#reading-details > summary').click();
  assert.equal(await page.locator('#reading-details').getAttribute('open'),'');
  await page.goto(new URL('/account#reading-history',base).href);
  assert.equal(await page.locator('#reading-details').getAttribute('open'),'');
  await page.reload();
  assert.equal(await page.locator('#reading-details').getAttribute('open'),'');
  await page.locator('[data-key-action=rotate]').first().click();
  await page.locator('#key-editor [name=confirm_rotation]').check();
  await page.locator('#key-editor form').evaluate(form => {
   window.accountSubmissions=[];
   form.addEventListener('submit',event => {
    event.preventDefault();
    window.accountSubmissions.push(new URLSearchParams(new FormData(form)).toString());
   });
  });
  await page.getByRole('button',{name:'Rotate API key',exact:true}).click();
  assert.equal(await page.locator('#key-editor button[type=submit]').isDisabled(),true);
  await page.locator('#key-editor button[type=submit]').evaluate(button => button.click());
  assert.deepEqual(await page.evaluate(() => window.accountSubmissions),['key_id=key_fixture_one&confirm_rotation=rotate']);
  await page.evaluate(() => window.dispatchEvent(new PageTransitionEvent('pageshow',{persisted:true})));
  assert.equal(await page.locator('#key-editor button[type=submit]').isEnabled(),true);
  for (const width of [320,390,768,1280]) {
   await page.setViewportSize({width,height:900});
   for (const state of ['active','error','unavailable']) {
    await page.goto(new URL('/login?state='+state,base).href);
    assert.equal(await page.getByRole('heading',{name:'Your research, connected.'}).isVisible(),true);
    assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),'Login overflow: '+width+' '+state);
    if(state==='unavailable') {
     assert.equal(await page.getByRole('link',{name:'Continue with Google'}).count(),0);
     assert.match(await page.getByRole('status').innerText(),/temporarily unavailable/);
    } else {
     assert.equal(await page.getByRole('link',{name:'Continue with Google'}).getAttribute('href'),'/auth/google/start?next=%2Faccount');
    }
    if(state==='error') assert.match(await page.getByRole('alert').innerText(),/could not start/);
    if([390,1280].includes(width)) await screenshot(`login-${width}-${state}.png`);
   }
  }
  assert.deepEqual(errors,[]);
  const noJS=await browser.newContext({javaScriptEnabled:false});
  const fallback=await noJS.newPage();
  await fallback.goto(new URL('/account',base).href);
  for (const id of ['create-key-fallback','rotate-key_fixture_two','revoke-key_fixture_two']) {
   await fallback.locator('#'+id+' summary').click();
   assert.equal(await fallback.locator('#'+id+' button[type=submit]').isVisible(),true);
  }
  await noJS.close();
  console.log('PASS: multiple keys and all account/login states at 320/390/768/1280px, correct per-key actions, create/rotate/revoke inline confirmations, reveal/copy, flat settings and reading deep links, duplicate-submit prevention, no-JS fallback, no page errors');
 } finally { await browser.close(); }
})().catch(error => {console.error(error);process.exitCode=1;});
