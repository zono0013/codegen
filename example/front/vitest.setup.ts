import '@testing-library/jest-dom'
import { notifyManager } from '@tanstack/react-query'

// TanStack Query のバッチ通知をテスト内で同期実行する
notifyManager.setScheduler(fn => fn())
