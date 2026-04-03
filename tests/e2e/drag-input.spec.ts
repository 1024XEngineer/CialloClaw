import { test, expect } from '@playwright/test';

test.describe('Drag Input Flow', () => {
  test('PDF happy path', async ({ page }) => {
    await page.goto('/');
    
    // Check initial state
    await expect(page.locator('h1')).toHaveText('对象即输入');
    await expect(page.getByLabel('悬浮球')).toHaveAttribute('data-status', 'idle');
    
    // Click 一键演示 button
    await page.click('button:has-text("一键演示")');
    
    // Wait for actions panel to appear
    await expect(page.locator('text=已识别：产品方案.pdf')).toBeVisible({ timeout: 8000 });
    
    // Click 总结 PDF action - use force to bypass viewport issues
    await page.click('button:has-text("总结 PDF")', { force: true });
    
    // Wait for processing to complete
    await expect(page.locator('text=PDF 总结')).toBeVisible({ timeout: 15000 });
    
    // Click 展开详情
    await page.click('button:has-text("展开详情")', { force: true });
    
    // Check detail view
    await expect(page.locator('h2:has-text("完整结果")')).toBeVisible();
  });

  test('unsupported file drag flow', async ({ page }) => {
    await page.goto('/');
    
    // Select zip file from tray
    await page.click('button:has-text("压缩包")');
    
    // Click 一键演示
    await page.click('button:has-text("一键演示")');
    
    // Should show unsupported message
    await expect(page.locator('text=暂不支持该格式')).toBeVisible({ timeout: 5000 });
    
    // Orb should show unsupported status
    await expect(page.getByLabel('悬浮球')).toHaveAttribute('data-eligibility', 'unsupported');
  });

  test('gallery state updates', async ({ page }) => {
    await page.goto('/');
    
    // Initial state - idle gallery item should be active
    const idleItem = page.locator('[data-testid="gallery-item-idle"]');
    await expect(idleItem).toHaveAttribute('data-active', 'true');
    
    // Click 一键演示
    await page.click('button:has-text("一键演示")');
    
    // Gallery should advance to actions
    const actionsItem = page.locator('[data-testid="gallery-item-actions"]');
    await expect(actionsItem).toHaveAttribute('data-active', 'true');
  });
});
