export function ctxFn(ctx: any, key: string) {
  return (...args: any[]) => ctx[key](...args)
}

export function ctxValue(ctx: any, key: string): any {
  return ctx[key]
}
