import { test, expect } from "@playwright/test";

test("register, play a game to resignation, see it in history", async ({ page }) => {
  const email = `e2e-${Date.now()}@example.com`;
  const password = "hunter2hunter2";

  await page.goto("/register");
  await page.getByLabel(/email/i).fill(email);
  await page.getByLabel(/password/i).fill(password);
  await page.getByRole("button", { name: /create account/i }).click();
  await expect(page).toHaveURL(/\/login/);

  await page.getByLabel(/email/i).fill(email);
  await page.getByLabel(/password/i).fill(password);
  await page.getByRole("button", { name: /log in/i }).click();
  await expect(page).toHaveURL(/\/play/);

  await page.getByRole("button", { name: /start game/i }).click();
  await expect(page).toHaveURL(/\/game\//);

  await page.getByRole("button", { name: /resign/i }).click();
  await expect(page.getByText(/game over: resigned/i)).toBeVisible();

  await page.goto("/games");
  await expect(page.getByText(/resigned/i).first()).toBeVisible();
});
