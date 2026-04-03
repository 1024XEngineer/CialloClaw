import { render, screen } from '@testing-library/react';
import App from '../App';

describe('FloatingOrb', () => {
  it('renders orb with correct initial state', () => {
    render(<App />);
    
    expect(screen.getByLabelText('悬浮球')).toHaveAttribute('data-eligibility', 'supported');
    expect(screen.getByLabelText('悬浮球')).toHaveAttribute('data-status', 'idle');
  });

  it('shows unsupported warning when zip is selected', async () => {
    render(<App />);
    
    const zipButton = screen.getByRole('button', { name: '压缩包' });
    zipButton.click();
    
    await screen.findByText('项目资料.zip');
    
    const orb = screen.getByLabelText('悬浮球');
    expect(orb).toHaveAttribute('data-eligibility', 'unsupported');
  });
});
