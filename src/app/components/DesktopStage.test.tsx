import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { traySamples } from '../model/mockData';
import App from '../App';

describe('DesktopStage', () => {
  it('changes the selected sample without auto-running the flow', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: '图片' }));

    expect(screen.getByText('白板拍照.png')).toBeInTheDocument();
    expect(screen.getByText('待拖入对象')).toBeInTheDocument();
    expect(screen.queryByText(/已识别：/)).not.toBeInTheDocument();
  });

  it('switches every sample without auto-running recognition or results', async () => {
    const user = userEvent.setup();
    render(<App />);

    for (const label of ['PDF', '图片', '文本', '链接', '压缩包']) {
      await user.click(screen.getByRole('button', { name: label }));
      expect(screen.getByTestId('gallery-item-idle')).toHaveAttribute('data-active', 'true');
      expect(screen.queryByText(/已识别：/)).not.toBeInTheDocument();
      expect(screen.queryByText('PDF 总结')).not.toBeInTheDocument();
    }
  });

  it('shows all five sample objects and resets the rail when switching samples', async () => {
    const user = userEvent.setup();
    render(<App />);

    expect(screen.getByText('产品方案.pdf')).toBeInTheDocument();
    expect(screen.getByText('白板拍照.png')).toBeInTheDocument();
    expect(screen.getByText('会议摘录.txt')).toBeInTheDocument();
    expect(screen.getByText('https://example.com/ai-desktop-workflow')).toBeInTheDocument();
    expect(screen.getByText('项目资料.zip')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: '一键演示' }));
    await user.click(screen.getByRole('button', { name: '图片' }));

    expect(screen.getByTestId('gallery-item-idle')).toHaveAttribute('data-active', 'true');
    expect(screen.queryByText('PDF 总结')).not.toBeInTheDocument();
  });

  it('can start a drag directly from the tray', async () => {
    render(<App />);

    const trayButton = screen.getByRole('button', { name: 'PDF' });
    fireEvent.pointerDown(trayButton, { clientX: 96, clientY: 88 });

    await waitFor(() => {
      expect(document.querySelector('.drag-ghost')).toBeInTheDocument();
    });
  });
});
