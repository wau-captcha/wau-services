# Deploy wau-captcha lên k3s (EC2)

k3s ≥ 1.30 trên 1 EC2 t4g.medium (ARM64, `ap-southeast-1`). Image trên ECR, database RDS, Redis ElastiCache Serverless; mọi thứ làm bằng tay với `kubectl`.

- `cluster/`: cấu hình cluster, làm một lần (Traefik, ClusterIssuer).
- `k3s/`: các app, `kubectl apply -k deploy/k3s`. `jobs/migrate.yaml` chạy riêng, không nằm trong kustomization.
- `scripts/build-push.sh <tag>`: build image `linux/arm64` rồi push lên ECR; chạy trên máy dev (cần AWS CLI, Docker buildx).
- `scripts/ecr-login.sh`: làm mới secret `ecr-pull`; chạy trên EC2 bằng cron.

Lệnh `kubectl` chạy trên EC2 với quyền root (`KUBECONFIG=/etc/rancher/k3s/k3s.yaml`), trong thư mục gốc của bản sao repo.

## Chạy lần đầu

0. Chuẩn bị:
   - 6 repository ECR `wau/{challenge,verify,tenant,billing,migrate,dashboard}`.
   - DNS `waucaptcha.com` và `api.waucaptcha.com` trỏ về IP của EC2; security group mở cổng 80 và 443.
   - Trên RDS: user `wau_app` là owner của database `wau_captcha`.
1. Cài k3s: `curl -sfL https://get.k3s.io | sh -`
2. Cấu hình Traefik giữ IP thật của người dùng: `sudo cp deploy/cluster/traefik-config.yaml /var/lib/rancher/k3s/server/manifests/`
3. Cài cert-manager rồi chờ sẵn sàng:
   ```sh
   kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.21.2/cert-manager.yaml
   kubectl -n cert-manager wait --for=condition=Available deployment --all --timeout=5m
   ```
4. Sửa email trong `deploy/cluster/cluster-issuer.yaml`, rồi `kubectl apply -f deploy/cluster/cluster-issuer.yaml`.
5. Tạo namespace và secret kéo image:
   ```sh
   kubectl create namespace wau
   sudo install -m 755 deploy/scripts/ecr-login.sh /usr/local/bin/ecr-login.sh && sudo ecr-login.sh
   ```
   Thêm vào crontab của root (`sudo crontab -e`): `0 */6 * * * /usr/local/bin/ecr-login.sh >> /var/log/ecr-login.log 2>&1`
6. Trên máy dev: `deploy/scripts/build-push.sh v1`.
7. Tạo Secret: chép `deploy/k3s/wau.env.example` thành `deploy/k3s/wau.env`, điền giá trị (file này bị git ignore), rồi
   `kubectl -n wau create secret generic wau-env --from-env-file=deploy/k3s/wau.env`. Sau đó xóa `wau.env` khỏi EC2.
8. Thay `ACCOUNT_ID` trong `deploy/k3s/kustomization.yaml` và `deploy/k3s/jobs/migrate.yaml`, rồi chạy migrate:
   ```sh
   kubectl apply -f deploy/k3s/jobs/migrate.yaml
   kubectl -n wau wait --for=condition=complete job/migrate --timeout=5m && kubectl -n wau logs job/migrate
   ```
9. `kubectl apply -k deploy/k3s`, rồi `kubectl -n wau get pods,certificate` cho tới khi pod Ready và chứng chỉ `READY True`.

## Cập nhật

1. `deploy/scripts/build-push.sh v2`
2. Đổi `newTag` trong `deploy/k3s/kustomization.yaml` (và tag image trong `jobs/migrate.yaml`) thành `v2`.
3. Nếu bản mới có migration, chạy migrate **trước** app (Job không chạy lại được, nên phải xóa Job cũ trước):
   ```sh
   kubectl -n wau delete job migrate --ignore-not-found
   kubectl apply -f deploy/k3s/jobs/migrate.yaml
   kubectl -n wau wait --for=condition=complete job/migrate --timeout=5m && kubectl -n wau logs job/migrate
   ```
4. `kubectl apply -k deploy/k3s`, rồi `kubectl -n wau rollout status deployment/<tên>`.

Đổi Secret: tạo lại bằng `... --from-env-file=deploy/k3s/wau.env --dry-run=client -o yaml | kubectl apply -f -`, rồi `kubectl -n wau rollout restart deployment`, vì pod chỉ đọc biến lúc khởi động.

## Rollback

`kubectl -n wau rollout undo deployment/<tên>`, rồi trả `newTag` trong `kustomization.yaml` về tag cũ để lần `apply -k` sau không deploy lại bản lỗi. Rollback app không gỡ migration đã chạy, nên migration phải tương thích với bản app trước đó.

## Lưu ý

- Database: app và migrate dùng user `wau_app`, không bao giờ dùng user master `postgres`. Nếu mật khẩu có ký tự đặc biệt thì phải mã hóa URL trong `KRATOS_DATABASE_SOURCE`, còn `PGPASSWORD` giữ nguyên.
- Redis là ElastiCache Serverless và luôn dùng TLS: `KRATOS_REDIS_TLS=true`. `KRATOS_REDIS_ADDRS` phải là endpoint dạng tên miền, vì chứng chỉ được kiểm theo tên đó. Không chạy Redis trong cluster.
- Không chạy seed trên production: Job chỉ chạy `goose up`; thư mục `/seed` có khóa API và admin mẫu.
- `api.waucaptcha.com` chỉ mở `/challenge/new`, `/challenge/verify`, `/siteverify` và `/v1/billing/sepay/ipn`. IPN khai ở SePay là `https://api.waucaptcha.com/v1/billing/sepay/ipn`. Không mở `/v1/*`: dashboard gọi tenant và billing trong cluster.
- Google OAuth client: *Authorized JavaScript origins* phải có `https://waucaptcha.com`.
- Đăng nhập (`POST /login`, `/admin/login`) bị giới hạn 20 lần/phút mỗi IP, tính riêng cho HTTPS và HTTP.
