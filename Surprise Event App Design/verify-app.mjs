import { chromium } from 'playwright';

const browser = await chromium.launch({
  executablePath: '/usr/bin/chromium',
  args: ['--no-sandbox', '--disable-setuid-sandbox'],
  headless: true
});
const page = await browser.newPage();
await page.setViewportSize({ width: 1400, height: 900 });

await page.goto('http://localhost:3000');
await page.waitForTimeout(1500);
await page.screenshot({ path: '/tmp/s1-registration.png' });
console.log('Screenshot 1: registration');

// Click "Create Account" button to go to main
await page.click('button:has-text("Create Account")');
await page.waitForTimeout(800);
await page.screenshot({ path: '/tmp/s2-main.png' });
console.log('Screenshot 2: main');

// Click profile button (top right circle with "A")
const profileBtn = await page.locator('button:has-text("A")').first();
await profileBtn.click();
await page.waitForTimeout(600);
await page.screenshot({ path: '/tmp/s3-profile.png' });
console.log('Screenshot 3: profile');

// Click "Generate Poll"
await page.click('button:has-text("Generate Poll")');
await page.waitForTimeout(600);
await page.screenshot({ path: '/tmp/s4-poll.png' });
console.log('Screenshot 4: poll');

// Select first option on poll
await page.locator('button:has-text("Outdoor Adventure")').click();
await page.waitForTimeout(300);
await page.click('button:has-text("Next Question")');
await page.waitForTimeout(400);
await page.screenshot({ path: '/tmp/s4b-poll-q2.png' });
console.log('Screenshot 4b: poll q2');

// Go back to main to test generate
await browser.close();

// Reopen for main → loader → results flow
const browser2 = await chromium.launch({
  executablePath: '/usr/bin/chromium',
  args: ['--no-sandbox', '--disable-setuid-sandbox'],
  headless: true
});
const page2 = await browser2.newPage();
await page2.setViewportSize({ width: 1400, height: 900 });
await page2.goto('http://localhost:3000');
await page2.waitForTimeout(1000);

// Register
await page2.click('button:has-text("Create Account")');
await page2.waitForTimeout(500);

// Select occasion
await page2.click('button:has-text("Select occasion")');
await page2.waitForTimeout(300);
await page2.click('button:has-text("Valentine\'s Day")');
await page2.waitForTimeout(300);

// Select budget
await page2.click('button:has-text("$50–$150")');
await page2.waitForTimeout(300);

// Click generate
await page2.click('button:has-text("Generate Magic")');
await page2.waitForTimeout(500);
await page2.screenshot({ path: '/tmp/s5-loader.png' });
console.log('Screenshot 5: loader');

// Wait for results
await page2.waitForTimeout(3500);
await page2.screenshot({ path: '/tmp/s6-results.png' });
console.log('Screenshot 6: results');

await browser2.close();
console.log('All screenshots captured');
