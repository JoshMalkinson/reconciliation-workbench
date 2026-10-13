import { test, expect } from "@playwright/test";
test("import, investigate, map, rerun, review, reopen and export", async ({
  page,
}) => {
  await page.goto("/");
  await page
    .getByRole("button", { name: "Try the sample portfolio", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "Portfolio reconciliation" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Inspect EXT-001" }).click();
  await page.getByLabel("Internal instrument").selectOption("FUND-A");
  await page.getByLabel("Reviewer name").fill("Josh");
  await page
    .getByLabel("Mapping explanation")
    .fill("Confirmed instrument identity with administrator reference.");
  await page
    .getByRole("button", { name: "Approve mapping", exact: true })
    .click();
  await expect(page.getByRole("status")).toContainText("Mapping approved");
  await page
    .getByRole("button", { name: "Rerun comparison", exact: true })
    .click();
  await page.getByRole("button", { name: "Inspect FUND-A" }).click();
  await expect(page.getByTestId("units-delta")).toContainText("−20");
  await expect(page.getByTestId("value-delta")).toContainText("−2");
  await page.getByLabel("Reviewer name").fill("Josh");
  await page
    .getByLabel("Review note")
    .fill("Trade settlement timing confirmed.");
  await page
    .getByRole("button", { name: "Mark reviewed", exact: true })
    .click();
  await expect(page.getByTestId("review-state")).toHaveText("Reviewed");
  await page.reload();
  await page.getByRole("button", { name: "Inspect FUND-A" }).click();
  await expect(
    page.getByText("Trade settlement timing confirmed.", { exact: true }),
  ).toBeVisible();
  await page.getByLabel("Reviewer name").fill("Josh");
  await page
    .getByLabel("Review note")
    .fill("Reopened pending final statement.");
  await page.getByRole("button", { name: "Reopen item", exact: true }).click();
  await expect(page.getByTestId("review-state")).toHaveText("Open");
  const download = page.waitForEvent("download");
  await page.getByRole("button", { name: "Export CSV", exact: true }).click();
  expect((await download).suggestedFilename()).toMatch(/^reconcile-.*\.csv$/);
  await page
    .getByRole("button", { name: /Open run/ })
    .nth(1)
    .click();
  await page.getByRole("button", { name: "Inspect FUND-A" }).click();
  await expect(page.getByTestId("classification")).toHaveText("Internal only");
  await page.screenshot({
    path: "test-results/workbench-desktop.png",
    fullPage: true,
  });
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.locator("body")).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBeTruthy();
  await page.screenshot({
    path: "test-results/workbench-mobile.png",
    fullPage: true,
  });
});
test("invalid uploads show location and imported markup stays literal", async ({
  page,
}) => {
  await page.goto("/");
  await page.getByRole("button", { name: "New import", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "Close import" }),
  ).toBeFocused();
  await page.keyboard.press("Shift+Tab");
  await expect(
    page.getByRole("button", { name: "Try the sample portfolio" }),
  ).toBeFocused();
  await page.keyboard.press("Tab");
  await expect(
    page.getByRole("button", { name: "Close import" }),
  ).toBeFocused();
  const internal =
    "instrument_id,instrument_name,as_of,currency,units,market_value\nA,<img src=x onerror=alert(1)>,2026-10-01,ZAR,1e3,20";
  const admin =
    "provider_code,security_id,description,valuation_date,ccy,quantity,value\nX,A,Example,2026-10-01,ZAR,1,20";
  await page.getByLabel("Internal holdings CSV").setInputFiles({
    name: "internal.csv",
    mimeType: "text/csv",
    buffer: Buffer.from(internal),
  });
  await page.getByLabel("Administrator CSV").setInputFiles({
    name: "administrator.csv",
    mimeType: "text/csv",
    buffer: Buffer.from(admin),
  });
  await page
    .getByRole("button", { name: "Compare files", exact: true })
    .click();
  await expect(page.getByRole("alert")).toContainText("row 2, units");
  await page.getByLabel("Internal holdings CSV").setInputFiles({
    name: "internal.csv",
    mimeType: "text/csv",
    buffer: Buffer.from(internal.replace("1e3", "1")),
  });
  await page
    .getByRole("button", { name: "Compare files", exact: true })
    .click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await page.getByLabel("Filter records").selectOption("all");
  await page.getByRole("button", { name: "Inspect A" }).click();
  await expect(
    page.getByRole("heading", { name: "<img src=x onerror=alert(1)>" }),
  ).toBeVisible();
  expect(await page.locator("img").count()).toBe(0);
});

test("stale tabs cannot overwrite another review", async ({
  page,
  context,
}) => {
  await page.goto("/");
  await page.getByRole("button", { name: "New import", exact: true }).click();
  await page.getByLabel("Internal holdings CSV").setInputFiles({
    name: "one.csv",
    mimeType: "text/csv",
    buffer: Buffer.from(
      "instrument_id,instrument_name,as_of,currency,units,market_value\nSTALE,Test holding,2026-10-01,ZAR,10,100",
    ),
  });
  await page.getByLabel("Administrator CSV").setInputFiles({
    name: "two.csv",
    mimeType: "text/csv",
    buffer: Buffer.from(
      "provider_code,security_id,description,valuation_date,ccy,quantity,value\nSTALE-EXT,STALE,Test holding,2026-10-01,ZAR,10,99",
    ),
  });
  await page
    .getByRole("button", { name: "Compare files", exact: true })
    .click();
  await expect(page.getByTestId("classification")).toHaveText(
    "Value difference",
  );
  const other = await context.newPage();
  await other.goto("/");
  await expect(other.getByTestId("review-state")).toHaveText("Open");
  await page.getByLabel("Reviewer name").fill("First reviewer");
  await page.getByLabel("Review note").fill("Confirmed statement timing");
  await page
    .getByRole("button", { name: "Mark reviewed", exact: true })
    .click();
  await expect(page.getByTestId("review-state")).toHaveText("Reviewed");
  await other.getByLabel("Reviewer name").fill("Second reviewer");
  await other.getByLabel("Review note").fill("Stale decision");
  await other
    .getByRole("button", { name: "Mark reviewed", exact: true })
    .click();
  await expect(other.getByRole("alert")).toContainText(
    "changed in another tab",
  );
  await other.getByRole("button", { name: "Reload run", exact: true }).click();
  await expect(other.getByTestId("review-state")).toHaveText("Reviewed");
  await expect(
    other.getByText("Confirmed statement timing", { exact: true }),
  ).toBeVisible();
  await expect(other.getByText("Stale decision", { exact: true })).toHaveCount(
    0,
  );
  await other.close();
});

test("a startup connection error can be recovered by reloading", async ({
  page,
}) => {
  await page.route("**/reconcile.v1.WorkbenchService/ListRuns", (route) =>
    route.abort(),
  );
  await page.goto("/");
  await expect(page.getByRole("alert")).toContainText(
    "Could not load saved runs",
  );
  await page.unroute("**/reconcile.v1.WorkbenchService/ListRuns");
  await page.reload();
  await expect(page.getByTestId("classification")).toBeVisible();
  await expect(page.getByRole("alert")).toHaveCount(0);
});

test("every new import starts with two fresh file selections", async ({
  page,
}) => {
  await page.goto("/");
  await page.getByRole("button", { name: "New import", exact: true }).click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Try the sample portfolio", exact: true })
    .click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await page.getByRole("button", { name: "New import", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "Compare files", exact: true }),
  ).toBeDisabled();
  await page
    .getByLabel("Internal holdings CSV")
    .setInputFiles({
      name: "selection.csv",
      mimeType: "text/csv",
      buffer: Buffer.from("unused"),
    });
  await page.getByRole("button", { name: "Close import" }).click();
  await page.getByRole("button", { name: "New import", exact: true }).click();
  await page
    .getByLabel("Administrator CSV")
    .setInputFiles({
      name: "selection.csv",
      mimeType: "text/csv",
      buffer: Buffer.from("unused"),
    });
  await expect(
    page.getByRole("button", { name: "Compare files", exact: true }),
  ).toBeDisabled();
});
