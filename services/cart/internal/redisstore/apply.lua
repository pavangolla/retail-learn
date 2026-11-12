-- apply.lua: change one cart line atomically.
--
-- Redis runs a script as one uninterrupted step, so no other client can slip a
-- command between our checks and our writes. That is what lets us enforce
-- "at most N lines" and "at most M per line" without a race, and make
-- "add to cart" safe to retry.
--
-- KEYS[1]  cart hash, field = sku, value = quantity
-- KEYS[2]  idempotency marker key (optional: only present for retryable adds)
-- ARGV[1]  mode: "add" (increase by qty) or "set" (replace with qty; 0 removes)
-- ARGV[2]  sku
-- ARGV[3]  qty
-- ARGV[4]  max quantity per line
-- ARGV[5]  max lines per cart
-- ARGV[6]  cart expiry, seconds (renewed on every change)
-- ARGV[7]  idempotency marker expiry, seconds
--
-- Returns {status, quantity}:
--   0 applied    1 duplicate request, nothing changed
--   2 too many lines    3 quantity above the per-line maximum
local cartKey = KEYS[1]
local mode, sku = ARGV[1], ARGV[2]
local qty = tonumber(ARGV[3])
local maxQty = tonumber(ARGV[4])
local maxLines = tonumber(ARGV[5])
local hasIdem = #KEYS >= 2

local current = tonumber(redis.call('HGET', cartKey, sku) or '0')

-- A repeat of a request we already applied: change nothing, report the quantity.
if hasIdem and redis.call('EXISTS', KEYS[2]) == 1 then
  return {1, current}
end

local target
if mode == 'add' then
  target = current + qty
else
  target = qty
end

if target > maxQty then
  return {3, current}
end
if target > 0 and current == 0 and redis.call('HLEN', cartKey) >= maxLines then
  return {2, current}
end

if target == 0 then
  redis.call('HDEL', cartKey, sku)
else
  redis.call('HSET', cartKey, sku, target)
end
redis.call('EXPIRE', cartKey, tonumber(ARGV[6]))

-- Record the marker only after success, so a rejected request can be retried.
if hasIdem then
  redis.call('SET', KEYS[2], '1', 'EX', tonumber(ARGV[7]))
end
return {0, target}
