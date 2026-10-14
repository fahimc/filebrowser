import assert from 'node:assert/strict';
import {chromium} from '../frontend/node_modules/@playwright/test/index.mjs';
import {mkdir} from 'node:fs/promises';
const targets=process.argv.slice(2);
const email=process.env.MYACCOUNTS_ADMIN_EMAIL,password=process.env.MYACCOUNTS_ADMIN_PASSWORD;
assert(targets.length&&email&&password);
const root=process.env.MYACCOUNTS_BROWSER_DATA;assert(root);
await mkdir(root,{recursive:true});
const context=await chromium.launchPersistentContext(root,{executablePath:process.env.OFFICE_BROWSER||'C:/Program Files/Google/Chrome/Application/chrome.exe',headless:true,viewport:{width:1365,height:900}});
let stableSubject;
try{
 for(const target of targets){
  const page=await context.newPage();await page.goto(new URL('/login',target).href);
  await page.getByRole('link',{name:'Sign in with MyAccounts'}).click();
  await page.waitForURL(u=>u.hostname==='login.m8e.co.uk');
  await page.locator('#username').waitFor({state:'visible'});
  const username=page.locator('#username');
  if(await username.isVisible().catch(()=>false)){
   await username.fill(email);
   if(!await page.locator('#password').isVisible())await page.getByRole('button',{name:'Next',exact:true}).click();
   await page.locator('#password').fill(password);
   if(process.env.MYACCOUNTS_TEST_REMEMBER==='1'&&await page.locator('#rememberIdentifier').isVisible())await page.locator('#rememberIdentifier').check();
   await page.getByRole('button',{name:'Sign In',exact:true}).click();
  }else{
   // A second relying party may offer the already authenticated account.
   await page.getByRole('button',{name:/continue|sign in/i}).first().click();
  }
  await page.waitForURL(u=>u.origin===new URL(target).origin,{timeout:90000});
  const response=await page.request.get(new URL('/api/users?username=self',target).href);
  assert.equal(response.status(),200,'FileBrowser browser callback did not establish a session');
  const user=await response.json();assert(user.username);if(stableSubject)assert.equal(user.username,stableSubject,'Clients did not link the same stable identity');else stableSubject=user.username;assert.equal(user.permissions.admin,false,'SSO incorrectly granted administrator privileges');
  const forbidden=await page.request.get(new URL('/api/users?username='+(target.startsWith('https:')?'remote-owner':'myaccounts-recovery-admin'),target).href);assert.equal(forbidden.status(),403,'Normal SSO user accessed another account');
  const cookies=await context.cookies(target);const session=cookies.find(c=>c.name==='filebrowser_quantum_jwt');assert(session?.httpOnly);if(target.startsWith('https:'))assert(session.secure);
  await page.screenshot({path:root+'/'+(target.startsWith('https:')?'remote':'local')+'-signed-in.png',fullPage:true});
  console.log('PASS: real browser HTTPS MyAccounts redirect, FileBrowser callback/session, normal-user permissions and protected cookie for '+new URL(target).protocol);
  await page.close();
 }
 if(process.env.MYACCOUNTS_TEST_REMEMBER==='1'){
  await context.clearCookies();const page=await context.newPage();await page.goto('https://login.m8e.co.uk/login');
  await page.getByRole('button',{name:email,exact:true}).click();await page.locator('#password').waitFor({state:'visible'});assert.equal(await page.locator('#password').inputValue(),'','Saved account stored a password');
  await page.getByRole('button',{name:'Change account',exact:true}).click();await page.getByRole('button',{name:'Remove saved account '+email,exact:true}).click();
  const identifiers=await page.evaluate(()=>JSON.parse(localStorage.getItem('myaccounts.saved-identifiers')||'[]'));assert.equal(identifiers.length,0);console.log('PASS: email-first sign-in, consented saved account, password-free selection, change/remove identifier');await page.close();
 }
}finally{await context.close();}
