package fr.clevertechware.demo.order;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import org.junit.jupiter.api.Test;
import org.springframework.boot.WebApplicationType;
import org.springframework.boot.builder.SpringApplicationBuilder;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;
import org.testcontainers.postgresql.PostgreSQLContainer;

@Testcontainers
class DatasourcePasswordConfigurationTest {

    @Container
    static PostgreSQLContainer postgres = new PostgreSQLContainer("postgres:18.6-trixie");

    @Test
    void startup_fails_on_authentication_when_datasource_password_is_not_provided() {
        assertThat(System.getenv("SPRING_DATASOURCE_PASSWORD")).isNull();

        assertThatThrownBy(() -> new SpringApplicationBuilder(OrderServiceApplication.class)
                .web(WebApplicationType.NONE)
                .run("--spring.datasource.url=" + postgres.getJdbcUrl(),
                        "--spring.datasource.username=" + postgres.getUsername()))
                .hasStackTraceContaining("password authentication failed");
    }

    @Test
    void startup_succeeds_when_datasource_password_is_provided() {
        try (var context = new SpringApplicationBuilder(OrderServiceApplication.class)
                .web(WebApplicationType.NONE)
                .run("--spring.datasource.url=" + postgres.getJdbcUrl(),
                        "--spring.datasource.username=" + postgres.getUsername(),
                        "--SPRING_DATASOURCE_PASSWORD=" + postgres.getPassword())) {
            assertThat(context.isRunning()).isTrue();
        }
    }
}
