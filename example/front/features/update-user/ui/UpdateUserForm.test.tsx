import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { User } from '@/generate/schemas/user.gen'
import { useUpdateUser } from '@/features/update-user/model/useUpdateUser'
import { UpdateUserForm } from './UpdateUserForm'

vi.mock('@/features/update-user/model/useUpdateUser')

const mockUser: User = {
  id: '1',
  name: '田中 太郎',
  email: 'tanaka@example.com',
  created_at: '2023-11-14T22:13:20Z',
}

const mockUpdate = vi.fn()

describe('UpdateUserForm', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(useUpdateUser).mockReturnValue({
      update: mockUpdate,
      isLoading: false,
      error: null,
    })
  })

  describe('保存ボタンの活性制御', () => {
    it('変更なしは保存ボタンが disabled', () => {
      render(<UpdateUserForm user={mockUser} />)
      expect(screen.getByRole('button', { name: '保存' })).toBeDisabled()
    })

    it('名前を変更すると保存ボタンが enabled になる', async () => {
      const user = userEvent.setup()
      render(<UpdateUserForm user={mockUser} />)

      await user.clear(screen.getByLabelText('名前'))
      await user.type(screen.getByLabelText('名前'), '鈴木 花子')

      expect(screen.getByRole('button', { name: '保存' })).toBeEnabled()
    })

    it('メールアドレスを変更すると保存ボタンが enabled になる', async () => {
      const user = userEvent.setup()
      render(<UpdateUserForm user={mockUser} />)

      await user.clear(screen.getByLabelText('メールアドレス'))
      await user.type(screen.getByLabelText('メールアドレス'), 'new@example.com')

      expect(screen.getByRole('button', { name: '保存' })).toBeEnabled()
    })

    it('名前を空にすると保存ボタンが disabled になる', async () => {
      const user = userEvent.setup()
      render(<UpdateUserForm user={mockUser} />)

      await user.clear(screen.getByLabelText('名前'))

      expect(screen.getByRole('button', { name: '保存' })).toBeDisabled()
    })
  })

  describe('送信', () => {
    it('保存ボタンを押すと update を正しい引数で呼ぶ', async () => {
      const user = userEvent.setup()
      mockUpdate.mockResolvedValue(mockUser)
      render(<UpdateUserForm user={mockUser} />)

      await user.clear(screen.getByLabelText('名前'))
      await user.type(screen.getByLabelText('名前'), '鈴木 花子')
      await user.click(screen.getByRole('button', { name: '保存' }))

      expect(mockUpdate).toHaveBeenCalledWith({
        id: '1',
        name: '鈴木 花子',
        email: 'tanaka@example.com',
      })
    })

    it('更新成功時に onSuccess を呼ぶ', async () => {
      const user = userEvent.setup()
      const onSuccess = vi.fn()
      mockUpdate.mockResolvedValue(mockUser)
      render(<UpdateUserForm user={mockUser} onSuccess={onSuccess} />)

      await user.clear(screen.getByLabelText('名前'))
      await user.type(screen.getByLabelText('名前'), '鈴木 花子')
      await user.click(screen.getByRole('button', { name: '保存' }))

      expect(onSuccess).toHaveBeenCalled()
    })

    it('更新失敗時は onSuccess を呼ばない', async () => {
      const user = userEvent.setup()
      const onSuccess = vi.fn()
      mockUpdate.mockResolvedValue(null)
      render(<UpdateUserForm user={mockUser} onSuccess={onSuccess} />)

      await user.clear(screen.getByLabelText('名前'))
      await user.type(screen.getByLabelText('名前'), '鈴木 花子')
      await user.click(screen.getByRole('button', { name: '保存' }))

      expect(onSuccess).not.toHaveBeenCalled()
    })
  })

  describe('エラー表示', () => {
    it('error state があるときエラーメッセージを表示する', () => {
      vi.mocked(useUpdateUser).mockReturnValue({
        update: mockUpdate,
        isLoading: false,
        error: 'このメールアドレスは既に使用されています',
      })

      render(<UpdateUserForm user={mockUser} />)

      expect(screen.getByText('このメールアドレスは既に使用されています')).toBeInTheDocument()
    })
  })
})
