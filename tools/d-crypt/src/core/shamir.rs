use anyhow::{Result, anyhow};
// Убрали reconstruct из импорта, он нам нужен как метод, а не функция
use sss_rs::wrapped_sharing::{share, Secret};

pub struct ShamirEngine;

impl ShamirEngine {
    /// Разделяет секрет на части
    pub fn split_secret(secret: &[u8], threshold: u8, total_shares: u8) -> Result<Vec<Vec<u8>>> {
        if threshold > total_shares {
            return Err(anyhow!("Threshold cannot be greater than total shares"));
        }

        // Создаем секрет для библиотеки
        let secret_obj = Secret::InMemory(secret.to_vec());

        // share возвращает Result<Vec<Vec<u8>>> - это уже байты!
        // Параметры: (secret, shares_required, shares_to_create, verify)
        let shares = share(secret_obj, threshold, total_shares, true)
            .map_err(|e| anyhow!("Shamir split failed: {:?}", e))?;

        // Просто возвращаем полученные шарды, конвертация не нужна
        Ok(shares)
    }

    /// Восстанавливает секрет
    pub fn recover_secret(shares_bytes: &[Vec<u8>], _threshold: u8) -> Result<Vec<u8>> {
        // Нам нужны Vec<Vec<u8>>, клонируем данные (так как sss-rs забирает владение)
        let shares_vec: Vec<Vec<u8>> = shares_bytes.to_vec();

        // Подготовка объекта для восстановления
        let mut secret_obj = Secret::empty_in_memory();
        
        // reconstruct(srcs: Vec<Vec<u8>>, verify: bool)
        // Добавили второй аргумент 'true' для проверки подписи
        secret_obj.reconstruct(shares_vec, true)
            .map_err(|e| anyhow!("Shamir recover failed: {:?}", e))?;

        // Извлекаем байты
        let secret_vec = secret_obj.unwrap_vec();
        
        Ok(secret_vec)
    }
}