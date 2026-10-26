import type { ReactNode } from 'react';
import { Tab, TabGroup, TabList, TabPanel, TabPanels } from '@headlessui/react';
import { cx } from './cx';

export interface TabItem {
  label: ReactNode;
  /** Count shown next to the label. */
  count?: number;
  /** Marks the tab (e.g. it holds an invalid field). */
  alert?: boolean;
  content: ReactNode;
}

/** Underlined tabs; panels stay mounted state-wise in the parent. */
export function Tabs({
  tabs,
  selectedIndex,
  onChange,
  className,
}: {
  tabs: TabItem[];
  selectedIndex?: number;
  onChange?: (index: number) => void;
  className?: string;
}) {
  return (
    <TabGroup selectedIndex={selectedIndex} onChange={onChange} className={className}>
      <TabList className="-mx-5 flex gap-5 overflow-x-auto border-b border-line px-5">
        {tabs.map((t, i) => (
          <Tab
            key={i}
            className={cx(
              'relative -mb-px flex shrink-0 items-center gap-1.5 border-b-2 border-transparent py-2.5 text-sm font-medium text-ink-muted transition-colors',
              'hover:text-ink focus:outline-none data-focus-visible:text-ink data-selected:border-primary-500 data-selected:text-ink',
            )}
          >
            {t.label}
            {t.count !== undefined && (
              <span className="rounded-full bg-surface-muted px-1.5 text-xs text-ink-muted tabular-nums">{t.count}</span>
            )}
            {t.alert && <span className="size-1.5 rounded-full bg-red-500" aria-label="has errors" />}
          </Tab>
        ))}
      </TabList>
      <TabPanels className="pt-5">
        {tabs.map((t, i) => (
          <TabPanel key={i} className="space-y-4 focus:outline-none">
            {t.content}
          </TabPanel>
        ))}
      </TabPanels>
    </TabGroup>
  );
}
