CREATE TABLE `wallets` (
  `id` INT AUTO_INCREMENT PRIMARY KEY,
  `owner` INT NOT NULL,
  `code` varchar(255) UNIQUE NOT NULL,
  `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  `balance` INT NOT NULL DEFAULT 0
);

CREATE TABLE `transfers` (
  `id` INT AUTO_INCREMENT PRIMARY KEY,
  `amount` INT NOT NULL,
  `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  `from_wallet_code` varchar(255) NOT NULL,
  `to_wallet_code` varchar(255) NOT NULL,
  CONSTRAINT fk_from_wallet FOREIGN KEY (from_wallet_code) REFERENCES wallets(code),
  CONSTRAINT fk_to_wallet FOREIGN KEY (to_wallet_code) REFERENCES wallets(code)
);