async (page) => {
  const logPrefix = '[playwrightTitleRegression 标题换行回归]';
  const expectedTitle = '企业 AI 标题换行回归';

  if ((await page.title()).indexOf(expectedTitle) < 0) {
    throw new Error(`${logPrefix} 当前页面不是由 testdata/title-wrap-regression.md 生成的回归页`);
  }

  // 三组固定场景覆盖日常桌面、日常移动端和用户可选择的最大字号。
  const cases = [
    { name: '桌面端默认字号', width: 1440, height: 900, fontSize: 17 },
    { name: '移动端默认字号', width: 390, height: 844, fontSize: 17 },
    { name: '移动端最大字号', width: 390, height: 844, fontSize: 24 },
  ];
  const results = [];

  for (const current of cases) {
    await page.setViewportSize({ width: current.width, height: current.height });
    await page.evaluate((fontSize) => localStorage.setItem('ainews:font-size', String(fontSize)), current.fontSize);
    await page.reload();
    await page.waitForTimeout(300);

    // 直接按浏览器实际坐标还原每一行，不根据字符数推测换行结果。
    const result = await page.evaluate(() => {
      const punctuationAtLineStart = /^[：:，,、。！？；.!?;]/;
      const singleHanLine = /^[\u3400-\u4dbf\u4e00-\u9fff\uf900-\ufaff]$/u;
      const headings = [...document.querySelectorAll('.article h2')].filter((heading) => /^\d+\./.test(heading.textContent.trim()));
      const allLines = [];
      let strongBreakCount = 0;
      const splitQuoteGroups = [];

      headings.forEach((heading) => {
        const rows = new Map();
        const groups = new Map();
        const words = [...heading.querySelectorAll('.heading-word')];

        words.forEach((word) => {
          const rect = word.getBoundingClientRect();
          const row = Math.round(rect.top);
          rows.set(row, (rows.get(row) || '') + word.textContent);
          (word.dataset.noBreakGroups || '').split(',').filter(Boolean).forEach((group) => {
            if (!groups.has(group)) groups.set(group, { rows: new Set(), width: 0 });
            groups.get(group).rows.add(row);
            groups.get(group).width += rect.width;
          });
          if (word.dataset.breakPriority === 'strong' && word.nextElementSibling?.classList.contains('heading-break')) {
            strongBreakCount += 1;
          }
        });

        allLines.push(...[...rows.entries()].sort((left, right) => left[0] - right[0]).map((entry) => entry[1]));
        const lineLimit = Math.max(1, heading.clientWidth - Math.max(2, heading.clientWidth * 0.01));
        groups.forEach((groupData, group) => {
          if (groupData.rows.size > 1 && groupData.width <= lineLimit) {
            splitQuoteGroups.push(`${heading.textContent.trim()}#${group}`);
          }
        });
      });

      return {
        headingCount: headings.length,
        overflowX: document.documentElement.scrollWidth > document.documentElement.clientWidth,
        punctuationLines: allLines.filter((line) => punctuationAtLineStart.test(line.trimStart())),
        singleHanLines: allLines.filter((line) => singleHanLine.test(line.trim())),
        strongBreakCount,
        splitQuoteGroups,
      };
    });

    const failures = [];
    if (result.headingCount !== 4) failures.push(`回归标题数量应为 4，实际为 ${result.headingCount}`);
    if (result.overflowX) failures.push('页面存在横向溢出');
    if (result.punctuationLines.length) failures.push(`存在标点开头行：${result.punctuationLines.join(' | ')}`);
    if (result.singleHanLines.length) failures.push(`存在单汉字孤行：${result.singleHanLines.join(' | ')}`);
    if (result.strongBreakCount < 1) failures.push('没有选择任何冒号或逗号后的高优先级断点');
    if (result.splitQuoteGroups.length) failures.push(`短引号词组被拆行：${result.splitQuoteGroups.join(' | ')}`);
    if (failures.length) throw new Error(`${logPrefix} ${current.name}失败：${failures.join('；')}`);

    results.push({ case: current.name, ...result });
  }

  await page.evaluate(() => localStorage.removeItem('ainews:font-size'));
  return { status: '通过', results };
}
