const UNIT_PRICES_IN_CENTS: [(&str, u64); 3] =
    [("SKU-001", 1_999), ("SKU-002", 4_550), ("SKU-003", 12_900)];

const DISCOUNT_TIERS: [(u32, u64); 3] = [(100, 15), (50, 10), (10, 5)];

pub fn unit_price(sku: &str, quantity: u32) -> Option<f64> {
    let base_cents = UNIT_PRICES_IN_CENTS
        .iter()
        .find(|(known_sku, _)| *known_sku == sku)
        .map(|(_, cents)| *cents)?;
    let discounted_cents = base_cents * (100 - discount_percent(quantity)) / 100;
    Some(discounted_cents as f64 / 100.0)
}

pub fn total(unit_price: f64, quantity: u32) -> f64 {
    (unit_price * 100.0).round() * f64::from(quantity) / 100.0
}

fn discount_percent(quantity: u32) -> u64 {
    DISCOUNT_TIERS
        .iter()
        .find(|(min_quantity, _)| quantity >= *min_quantity)
        .map_or(0, |(_, percent)| *percent)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn unknown_sku_has_no_price() {
        assert_eq!(unit_price("SKU-999", 1), None);
    }

    #[test]
    fn total_multiplies_unit_price_by_quantity_in_cents() {
        assert_eq!(total(19.99, 3), 59.97);
    }

    #[test]
    fn small_quantity_pays_the_list_price() {
        assert_eq!(unit_price("SKU-001", 9), Some(19.99));
    }

    #[test]
    fn ten_units_get_five_percent_off() {
        assert_eq!(unit_price("SKU-003", 10), Some(122.55));
    }

    #[test]
    fn fifty_units_get_ten_percent_off() {
        assert_eq!(unit_price("SKU-002", 50), Some(40.95));
    }

    #[test]
    fn one_hundred_units_get_fifteen_percent_off() {
        assert_eq!(unit_price("SKU-003", 100), Some(109.65));
    }
}
