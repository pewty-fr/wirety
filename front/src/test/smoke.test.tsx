import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { seedNetwork } from './fakeApi';
import { renderApp } from './renderApp';

describe('app shell', () => {
  it('renders the dashboard for the signed-in admin', async () => {
    seedNetwork({ name: 'corp' });
    renderApp('/dashboard');
    expect(await screen.findByRole('heading', { name: 'Dashboard' })).toBeInTheDocument();
  });
});
