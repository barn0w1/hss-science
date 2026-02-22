FROM migrate/migrate
COPY . /migrations/
# The orchestrator will inject DATABASE_URL at runtime
ENTRYPOINT ["migrate", "-path", "/migrations", "-database"]
CMD ["${DATABASE_URL}", "up"]
