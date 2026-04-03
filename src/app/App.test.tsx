import { render, screen } from '@testing-library/react';
import App from './App';

it('renders the desktop stage shell', () => {
  render(<App />);
  expect(screen.getByRole('heading', { name: '对象即输入' })).toBeInTheDocument();
  expect(screen.getByLabelText('桌面交互主舞台')).toBeInTheDocument();
  expect(screen.getByLabelText('右侧上下文栏')).toBeInTheDocument();
});
