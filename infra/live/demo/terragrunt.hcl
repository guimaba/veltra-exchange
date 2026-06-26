include "root" {
  path = find_in_parent_folders()
}

terraform {
  source = "${dirname(find_in_parent_folders())}//stack"
}

# Ambiente DEMO: catálogo completo de pares (apresentação), RDS um degrau acima.
inputs = {
  env                = "demo"
  image_tag          = "latest"
  pairs              = "VLT/USDT-sim,BTC/USDT-sim,ETH/USDT-sim,BNB/USDT-sim,SOL/USDT-sim,XRP/USDT-sim,ADA/USDT-sim,DOGE/USDT-sim,DOT/USDT-sim,AVAX/USDT-sim,LINK/USDT-sim,UNI/USDT-sim,LTC/USDT-sim,ATOM/USDT-sim,NEAR/USDT-sim"
  rds_instance_class = "db.t4g.small"
  mq_instance_type   = "mq.t3.micro"
  enable_nat         = false
}
