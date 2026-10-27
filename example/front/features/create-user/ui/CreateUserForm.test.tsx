import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { toast } from 'sonner'
import type { User } from '@/generate/schemas/user.gen'
import { useCreateUser } from '@/features/create-user/model/useCreateUser'
import { CreateUserForm } from './CreateUserForm'

vi.mock('@/features/create-user/model/useCreateUser')
vi.mock('sonner')
vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: mockPush }),
}))

const mockPush = vi.fn()
const mockCreate = vi.fn()

const mockUser: User = {
  id: '1',
  name: '田中 太郎',
  email: 'tanaka@example.com',
  created_at: '2023-11-14T22:13:20Z',
}

describe('CreateUserForm', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(useCreateUser).mockReturnValue({
      create: mockCreate,
      isLoading: false,
      error: null,
    })
  })

  describe('追加ボタンの活性制御', () => {
    it('初期状態では追加ボタンが disabled', () => {
      render(<CreateUserForm />)
      expect(screen.getByRole('button', { name: '追加' })).toBeDisabled()
    })

    it('名前のみ入力しても追加ボタンが disabled のまま', async () => {
      const user = userEvent.setup()
      render(<CreateUserForm />)

      await user.type(screen.getByLabelText('名前'), '田中 太郎')

      expect(screen.getByRole('button', { name: '追加' })).toBeDisabled()
    })

    it('名前とメールアドレスを両方入力すると追加ボタンが enabled になる', async () => {
      const user = userEvent.setup()
      render(<CreateUserForm />)

      await user.type(screen.getByLabelText('名前'), '田中 太郎')
      await user.type(screen.getByLabelText('メールアドレス'), 'tanaka@example.com')

      expect(screen.getByRole('button', { name: '追加' })).toBeEnabled()
    })
  })

  describe('送信', () => {
    it('追加ボタンを押すと create を正しい引数で呼ぶ', async () => {
      const user = userEvent.setup()
      mockCreate.mockResolvedValue(mockUser)
      render(<CreateUserForm />)

      await user.type(screen.getByLabelText('名前'), '田中 太郎')
      await user.type(screen.getByLabelText('メールアドレス'), 'tanaka@example.com')
      await user.click(screen.getByRole('button', { name: '追加' }))

      expect(mockCreate).toHaveBeenCalledWith({
        name: '田中 太郎',
        email: 'tanaka@example.com',
      })
    })

    it('作成成功時に toast.success を表示して /users に遷移する', async () => {
      const user = userEvent.setup()
      mockCreate.mockResolvedValue(mockUser)
      render(<CreateUserForm />)

      await user.type(screen.getByLabelText('名前'), '田中 太郎')
      await user.type(screen.getByLabelText('メールアドレス'), 'tanaka@example.com')
      await user.click(screen.getByRole('button', { name: '追加' }))

      expect(toast.success).toHaveBeenCalledWith('田中 太郎 を追加しました')
      expect(mockPush).toHaveBeenCalledWith('/users')
    })

    it('作成失敗時は toast も遷移も発生しない', async () => {
      const user = userEvent.setup()
      mockCreate.mockResolvedValue(null)
      render(<CreateUserForm />)

      await user.type(screen.getByLabelText('名前'), '田中 太郎')
      await user.type(screen.getByLabelText('メールアドレス'), 'tanaka@example.com')
      await user.click(screen.getByRole('button', { name: '追加' }))

      expect(toast.success).not.toHaveBeenCalled()
      expect(mockPush).not.toHaveBeenCalledWith('/users')
    })
  })

  describe('キャンセル', () => {
    it('キャンセルボタンを押すと /users に遷移する', async () => {
      const user = userEvent.setup()
      render(<CreateUserForm />)

      await user.click(screen.getByRole('button', { name: 'キャンセル' }))

      expect(mockPush).toHaveBeenCalledWith('/users')
    })
  })

  describe('エラー表示', () => {
    it('error state があるときエラーメッセージを表示する', () => {
      vi.mocked(useCreateUser).mockReturnValue({
        create: mockCreate,
        isLoading: false,
        error: 'このメールアドレスは既に使用されています',
      })

      render(<CreateUserForm />)

      expect(screen.getByText('このメールアドレスは既に使用されています')).toBeInTheDocument()
    })
  })
})
