# Deploy wau-captcha lên k3s (EC2)

k3s ≥ 1.30 trên 1 EC2 t4g.medium (ARM64, `ap-southeast-1`). Image trên ECR, database RDS, Redis ElastiCache Serverless. GitHub Actions build image, Argo CD đồng bộ `deploy/k3s` từ nhánh `main` (GitOps); phần cài đặt ban đầu vẫn làm bằng tay với `kubectl`.

- `cluster/`: cấu hình cluster, làm một lần (Traefik, ClusterIssuer).
- `k3s/`: các app và Job `migrate.yaml` (PreSync hook của Argo CD); Argo CD đồng bộ thư mục này.
- `argocd/wau.yaml`: Application `wau` của Argo CD; `kubectl apply -f` một lần, không nằm trong kustomization.
- `scripts/build-push.sh <tag>`: build image `linux/arm64` rồi push lên ECR từ máy dev (cần AWS CLI, Docker buildx); chỉ dùng khi không qua GitHub Actions.
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
8. Cài Argo CD, rồi cho nó đọc repo (repo private; dùng một token chỉ đọc Contents của wau-services, không commit):
   ```sh
   kubectl create namespace argocd
   kubectl apply -n argocd --server-side --force-conflicts -f https://raw.githubusercontent.com/argoproj/argo-cd/v3.5.3/manifests/install.yaml
   kubectl -n argocd wait --for=condition=Available deployment --all --timeout=5m
   kubectl -n argocd create secret generic wau-services-repo --from-literal=type=git \
     --from-literal=url=https://github.com/wau-captcha/wau-services.git --from-literal=username=x --from-literal=password=<TOKEN>
   kubectl -n argocd label secret wau-services-repo argocd.argoproj.io/secret-type=repository
   ```
9. `kubectl apply -f deploy/argocd/wau.yaml`. Argo CD chạy Job `migrate` trước rồi mới tới các app. Theo dõi bằng `kubectl -n argocd get application wau` (chờ `Synced` và `Healthy`) và `kubectl -n wau get pods,certificate`, cho tới khi pod Ready và chứng chỉ `READY True`.

## Cập nhật

1. Mở PR: CI chạy gofmt, vet, build, test cho 4 service và render `deploy/k3s`; ở wau-dashboard là lint và build.
2. Merge vào `main`:
   - wau-services, khi đổi `services/**` hoặc `infra/postgres/**`: build 4 service và migrate, push tag `sha-<7 ký tự đầu của commit>`, rồi bot commit `chore(deploy): release sha-... [skip ci]` đặt `newTag` của challenge, verify, tenant, billing và migrate.
   - wau-dashboard: build dashboard (arm64 chạy giả lập nên chậm), rồi bot commit vào wau-services `chore(deploy): release dashboard sha-... [skip ci]`, chỉ đổi `newTag` của dashboard.
3. Argo CD thấy commit mới trong khoảng 3 phút và chạy Job `migrate` trước. Migrate lỗi thì sync dừng, app giữ bản đang chạy; xem `kubectl -n wau logs job/migrate`.
4. Sau mỗi bản phát hành, `git pull` để lấy commit tag của bot trước khi sửa tiếp.

Không chạy `kubectl apply -k deploy/k3s` bằng tay nữa: self-heal của Argo CD trả mọi thay đổi tay về đúng `main`.

Đổi Secret: tạo lại bằng `... --from-env-file=deploy/k3s/wau.env --dry-run=client -o yaml | kubectl apply -f -`, rồi `kubectl -n wau rollout restart deployment`, vì pod chỉ đọc biến lúc khởi động. Argo CD không quản lý Secret này nên không tự restart.

`deploy/scripts/build-push.sh <tag>` vẫn dùng được, nhưng tag đó phải được commit vào `newTag` trong `deploy/k3s/kustomization.yaml` trên `main` thì Argo CD mới deploy.

## Rollback

Trên `main`, đổi `newTag` về tag cũ (image `sha-...` cũ vẫn còn trên ECR) hoặc `git revert` commit release của bot, rồi push; Argo CD sync lại như một bản phát hành. Không dùng nút Rollback trên UI Argo CD khi auto-sync đang bật: Argo CD không cho, và auto-sync sẽ đưa mọi thứ về lại `main`. Rollback app không gỡ migration đã chạy, nên migration phải tương thích với bản app trước đó.

## Lưu ý

- Database: app và migrate dùng user `wau_app`, không bao giờ dùng user master `postgres`. Nếu mật khẩu có ký tự đặc biệt thì phải mã hóa URL trong `KRATOS_DATABASE_SOURCE`, còn `PGPASSWORD` giữ nguyên.
- Redis là ElastiCache Serverless và luôn dùng TLS: `KRATOS_REDIS_TLS=true`. `KRATOS_REDIS_ADDRS` phải là endpoint dạng tên miền, vì chứng chỉ được kiểm theo tên đó. Không chạy Redis trong cluster.
- Không chạy seed trên production: Job chỉ chạy `goose up`; thư mục `/seed` có khóa API và admin mẫu.
- `api.waucaptcha.com` chỉ mở `/challenge/new`, `/challenge/verify`, `/siteverify` và `/v1/billing/sepay/ipn`. IPN khai ở SePay là `https://api.waucaptcha.com/v1/billing/sepay/ipn`. Không mở `/v1/*`: dashboard gọi tenant và billing trong cluster.
- Google OAuth client: *Authorized JavaScript origins* phải có `https://waucaptcha.com`.
- Đăng nhập (`POST /login`, `/admin/login`) bị giới hạn 20 lần/phút mỗi IP, tính riêng cho HTTPS và HTTP.
