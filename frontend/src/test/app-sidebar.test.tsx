import { act, fireEvent, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, beforeEach, expect, test, vi } from 'vitest';

import { useAllSettings } from '@/api/queries/useAllSettings';
import AppSidebar from '@/layouts/AppSidebar';
import { renderWithProviders } from './test-utils';

vi.mock('@/api/queries/useAllSettings', () => ({
  useAllSettings: vi.fn(() => ({ allSetting: {} })),
}));

const defaultSettings = () => ({ allSetting: {} }) as unknown as ReturnType<typeof useAllSettings>;

beforeEach(() => {
  vi.mocked(useAllSettings).mockReturnValue(defaultSettings());
});

afterEach(() => {
  localStorage.clear();
});

function topLevelLabels(view: { container: HTMLElement }) {
  return Array.from(view.container.querySelectorAll('.sider-nav > li')).map(
    (li) => li.textContent ?? '',
  );
}

// rc-menu registers its items in a microtask after render; settle it inside act().
async function renderSidebar() {
  const view = renderWithProviders(
    <MemoryRouter>
      <AppSidebar />
    </MemoryRouter>,
  );
  await act(async () => {});
  return view;
}

test('keeps the sidebar expanded after pinning it from the header and restores the choice', async () => {
  const first = await renderSidebar();
  const sidebar = first.container.querySelector('.ant-layout-sider');
  const sidebarRoot = first.container.querySelector('.ant-sidebar');

  expect(sidebar?.classList.contains('ant-layout-sider-collapsed')).toBe(true);

  fireEvent.mouseEnter(sidebarRoot!);

  const pinButton = screen.getByRole('button', { name: 'Pin sidebar' });
  expect(pinButton.closest('.brand-actions')).not.toBeNull();

  fireEvent.click(pinButton);
  fireEvent.mouseLeave(sidebarRoot!);

  expect(sidebar?.classList.contains('ant-layout-sider-collapsed')).toBe(false);
  expect(sidebarRoot?.getAttribute('style')).toContain('--sider-rail: 220px');
  expect(localStorage.getItem('sidebar-pinned')).toBe('true');

  first.unmount();

  const second = await renderSidebar();
  const restoredSidebar = second.container.querySelector('.ant-layout-sider');
  const restoredSidebarRoot = second.container.querySelector('.ant-sidebar');

  expect(restoredSidebar?.classList.contains('ant-layout-sider-collapsed')).toBe(false);
  expect(restoredSidebarRoot?.getAttribute('style')).toContain('--sider-rail: 220px');
  expect(screen.getByRole('button', { name: 'Pin sidebar' })).not.toBeNull();
});

test('returns to the compact rail after unpinning', async () => {
  const view = await renderSidebar();
  const sidebar = view.container.querySelector('.ant-layout-sider');
  const sidebarRoot = view.container.querySelector('.ant-sidebar');

  fireEvent.mouseEnter(sidebarRoot!);
  fireEvent.click(screen.getByRole('button', { name: 'Pin sidebar' }));
  fireEvent.click(screen.getByRole('button', { name: 'Pin sidebar' }));
  fireEvent.mouseLeave(sidebarRoot!);

  expect(sidebar?.classList.contains('ant-layout-sider-collapsed')).toBe(true);
  expect(sidebarRoot?.getAttribute('style')).toContain('--sider-rail: 72px');
  expect(localStorage.getItem('sidebar-pinned')).toBe('false');
});

test('labels the palette shortcut with the modifier the platform actually uses', async () => {
  const view = await renderSidebar();
  const chip = view.container.querySelector('.sidebar-command-kbd');
  expect(chip?.textContent).toBe('CtrlK');
});

test('subscription settings is a top-level entry directly below rules', async () => {
  const view = await renderSidebar();
  const labels = topLevelLabels(view);
  const rules = labels.findIndex((label) => label.includes('Rules'));

  expect(rules).toBeGreaterThanOrEqual(0);
  expect(labels[rules + 1]).toContain('Subscription');
  expect(labels[rules + 2]).toContain('Panel Settings');
});

test('sub formats joins the top level below rules when a format is enabled', async () => {
  vi.mocked(useAllSettings).mockReturnValue({
    allSetting: { subJsonEnable: true },
  } as unknown as ReturnType<typeof useAllSettings>);

  const view = await renderSidebar();
  const labels = topLevelLabels(view);
  const rules = labels.findIndex((label) => label.includes('Rules'));

  expect(rules).toBeGreaterThanOrEqual(0);
  expect(labels[rules + 1]).toContain('Subscription');
  expect(labels[rules + 2]).toContain('Sub Formats');
  expect(labels[rules + 3]).toContain('Panel Settings');
});
